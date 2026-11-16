package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// generatorVersion is part of the fixture five-tuple. Any change to layout or
// record content must change it.
const generatorVersion = "fixture/1"

const (
	codexShare     = 0.60
	archivedShare  = 0.15
	malformedRate  = 0.001
	overlongLine   = 8 * mib
	subagentShare  = 0.10
	projectCount   = 40
	todayFirstMin  = 1  // minutes after local midnight
	todayLastMin   = 44 // a batch never starts within 45 minutes of midnight
	continuedSplit = 0.7
	streamLayout   = 0x6c61796f7574 // "layout"
	streamContent  = 0x636f6e74656e // "conten"
)

type fixtureOptions struct {
	profile  profile
	seed     uint64
	anchor   time.Time // local midnight of the anchor date
	location *time.Location
	out      string
}

type fileSpec struct {
	index     int
	rel       string
	size      int64
	client    string
	archived  bool
	today     bool
	continued bool
	overlong  bool
	subagent  bool
	session   string
	project   int
	start     time.Time
	end       time.Time
	// todayStart and todayEnd bound the today part of a continued session.
	todayStart, todayEnd time.Time
}

// fixtureManifest is printed after generation; it is the fixture identity
// recorded with every batch.
type fixtureManifest struct {
	GeneratorVersion string  `json:"generator_version"`
	Profile          string  `json:"profile"`
	Seed             uint64  `json:"seed"`
	AnchorDate       string  `json:"anchor_date"`
	Timezone         string  `json:"timezone"`
	Identity         string  `json:"identity"`
	Sigma            float64 `json:"sigma"`
	MedianBytes      int64   `json:"median_bytes"`
	Files            int     `json:"files"`
	TotalBytes       int64   `json:"total_bytes"`
	TodayFiles       int     `json:"today_files"`
	TodayBytes       int64   `json:"today_bytes"`
	TodayContinued   int     `json:"today_continued"`
	LargeFiles       int     `json:"large_files"`
}

func runFixture(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("fixture", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profileName := flags.String("profile", "small", "synthetic fixture profile (small; tests only)")
	seed := flags.Uint64("seed", 1, "fixture seed")
	anchor := flags.String("anchor", "", "anchor date YYYY-MM-DD in --tz")
	tz := flags.String("tz", "America/New_York", "IANA timezone of the anchor date")
	out := flags.String("out", "", "empty output directory that becomes the fixture HOME")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	opts, err := parseFixtureOptions(*profileName, *seed, *anchor, *tz, *out)
	if err == nil {
		var manifest fixtureManifest
		if manifest, err = generateFixture(opts); err == nil {
			encoder := json.NewEncoder(stdout)
			encoder.SetIndent("", "  ")
			err = encoder.Encode(manifest)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "agentdeck-perf fixture:", err)
		return 1
	}
	return 0
}

func parseFixtureOptions(name string, seed uint64, anchor, tz, out string) (fixtureOptions, error) {
	p, ok := profiles[name]
	if !ok {
		return fixtureOptions{}, fmt.Errorf("unknown profile %q", name)
	}
	location, err := time.LoadLocation(tz)
	if err != nil {
		return fixtureOptions{}, err
	}
	day, err := time.ParseInLocation("2006-01-02", anchor, location)
	if err != nil {
		return fixtureOptions{}, fmt.Errorf("--anchor: %w", err)
	}
	if out == "" || !filepath.IsAbs(out) {
		return fixtureOptions{}, errors.New("--out must be an absolute path")
	}
	return fixtureOptions{profile: p, seed: seed, anchor: day, location: location, out: out}, nil
}

func generateFixture(opts fixtureOptions) (fixtureManifest, error) {
	if err := requireEmptyDir(opts.out); err != nil {
		return fixtureManifest{}, err
	}
	plan, err := planSizes(opts.profile)
	if err != nil {
		return fixtureManifest{}, err
	}
	specs := layoutFiles(opts, plan)
	if err := writeFiles(opts, specs); err != nil {
		return fixtureManifest{}, err
	}
	// Spotlight must not index the fixture while a batch runs.
	if err := os.WriteFile(filepath.Join(opts.out, ".metadata_never_index"), nil, 0o600); err != nil {
		return fixtureManifest{}, err
	}
	identity, err := fixtureIdentity(opts.out)
	if err != nil {
		return fixtureManifest{}, err
	}
	large := mib * 4
	manifest := fixtureManifest{
		GeneratorVersion: generatorVersion, Profile: opts.profile.name, Seed: opts.seed,
		AnchorDate: opts.anchor.Format("2006-01-02"), Timezone: opts.location.String(),
		Identity: identity, Sigma: plan.sigma, MedianBytes: plan.medianSize(),
		Files: len(specs), TodayFiles: opts.profile.todayFiles, TodayBytes: plan.todayBytes,
		TodayContinued: opts.profile.todayContinued,
	}
	for _, spec := range specs {
		manifest.TotalBytes += spec.size
		if spec.size >= large {
			manifest.LargeFiles++
		}
	}
	return manifest, nil
}

func requireEmptyDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("--out must be empty")
	}
	return nil
}

// layoutFiles implements step 5: the seed assigns sizes to paths, clients,
// archive placement and continued sessions without changing either size set.
func layoutFiles(opts fixtureOptions, plan sizePlan) []fileSpec {
	p := opts.profile
	rng := rand.New(rand.NewPCG(opts.seed, streamLayout))
	k := p.todayFiles
	today := append([]int64(nil), plan.rest[plan.todayStart:plan.todayStart+k]...)
	others := append(append([]int64(nil), plan.rest[:plan.todayStart]...), plan.rest[plan.todayStart+k:]...)
	others = append(others, p.fixed...)
	rng.Shuffle(len(today), func(i, j int) { today[i], today[j] = today[j], today[i] })
	rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })

	clients := func(n int) []string {
		out := make([]string, n)
		codex := int(math.Round(codexShare * float64(n)))
		for i := range out {
			out[i] = "claude"
			if i < codex {
				out[i] = "codex"
			}
		}
		rng.Shuffle(n, func(i, j int) { out[i], out[j] = out[j], out[i] })
		return out
	}
	todayClients, otherClients := clients(k), clients(len(others))

	specs := make([]fileSpec, 0, p.files)
	for i, size := range today {
		specs = append(specs, fileSpec{size: size, client: todayClients[i], today: true, continued: i < p.todayContinued})
	}
	largest := 0
	for i, size := range others {
		if size > others[largest] {
			largest = i
		}
		specs = append(specs, fileSpec{size: size, client: otherClients[i]})
	}
	specs[k+largest].overlong = true

	var oldCodex []int
	codexTotal := 0
	for i := range specs {
		if specs[i].client == "codex" {
			codexTotal++
			if !specs[i].today {
				oldCodex = append(oldCodex, i)
			}
		}
	}
	rng.Shuffle(len(oldCodex), func(i, j int) { oldCodex[i], oldCodex[j] = oldCodex[j], oldCodex[i] })
	for _, i := range oldCodex[:int(math.Round(archivedShare*float64(codexTotal)))] {
		specs[i].archived = true
	}

	used := map[string]bool{}
	for i := range specs {
		spec := &specs[i]
		spec.index = i
		spec.session = uniqueUUID(rng, used)
		spec.project = rng.IntN(projectCount)
		spec.subagent = spec.client == "claude" && rng.Float64() < subagentShare
		midnight := opts.anchor
		todayFrom := midnight.Add(time.Duration(todayFirstMin) * time.Minute)
		todayTo := midnight.Add(time.Duration(todayLastMin) * time.Minute)
		switch {
		case spec.today && !spec.continued:
			spec.start = todayFrom.Add(time.Duration(rng.IntN(15*60)) * time.Second)
			spec.end = todayTo
		default:
			day := midnight.AddDate(0, 0, -(1 + rng.IntN(p.spanDays)))
			spec.start = day.Add(time.Duration(8*3600+rng.IntN(12*3600)) * time.Second)
			spec.end = day.Add(23*time.Hour + 50*time.Minute)
			if spec.continued {
				spec.todayStart, spec.todayEnd = todayFrom, todayTo
			}
		}
		spec.rel = fixturePath(*spec, opts.location, rng, used)
	}
	return specs
}

func uniqueUUID(rng *rand.Rand, used map[string]bool) string {
	for {
		var b [16]byte
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		b[6] = b[6]&0x0f | 0x40
		b[8] = b[8]&0x3f | 0x80
		id := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
		if !used[id] {
			used[id] = true
			return id
		}
	}
}

func fixturePath(spec fileSpec, location *time.Location, rng *rand.Rand, used map[string]bool) string {
	local := spec.start.In(location)
	stamp := local.Format("2006-01-02T15-04-05")
	if spec.client == "codex" {
		name := "rollout-" + stamp + "-" + spec.session + ".jsonl"
		if spec.archived {
			return filepath.Join(".codex", "archived_sessions", name)
		}
		return filepath.Join(".codex", "sessions", local.Format("2006"), local.Format("01"), local.Format("02"), name)
	}
	project := fmt.Sprintf("-perf-workspace-p%02d", spec.project)
	if spec.subagent {
		parent := uniqueUUID(rng, used)
		return filepath.Join(".claude", "projects", project, parent, "subagents", "agent-"+spec.session[:8]+".jsonl")
	}
	return filepath.Join(".claude", "projects", project, spec.session+".jsonl")
}

func writeFiles(opts fixtureOptions, specs []fileSpec) error {
	jobs := make(chan fileSpec)
	errs := make(chan error, len(specs))
	var wait sync.WaitGroup
	for range max(1, runtime.NumCPU()) {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for spec := range jobs {
				errs <- writeFixtureFile(opts, spec)
			}
		}()
	}
	for _, spec := range specs {
		jobs <- spec
	}
	close(jobs)
	wait.Wait()
	close(errs)
	var all []error
	for err := range errs {
		if err != nil {
			all = append(all, err)
		}
	}
	return errors.Join(all...)
}

func writeFixtureFile(opts fixtureOptions, spec fileSpec) error {
	path := filepath.Join(opts.out, spec.rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	w := bufio.NewWriterSize(file, 1<<20)
	last, err := generateContent(w, opts.seed, spec)
	if err == nil {
		err = w.Flush()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Chtimes(path, last, last)
}

// fixtureIdentity hashes the sorted relative_path:size:sha256 lines joined by
// LF with no trailing LF.
func fixtureIdentity(root string) (string, error) {
	var lines []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		hash := sha256.New()
		size, err := io.Copy(hash, file)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), size, hex.EncodeToString(hash.Sum(nil))))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

---
status: active
created: 2026-08-27
updated: 2026-10-02
---

# Brand Asset Provenance

## Current AD shared-stroke mark

The operator selected the orange AD shared-stroke mark with a dark app tile
and two cuts in the D ring. Issue #29 imports exact PNG bytes from
`AgentDeck-Icon-Review-v1.zip` (Library
`libfile_18670278130c8191a708e0c61daa5d14`), SHA-256
`83e01d3bd0956fb6b31209415cd20400194efb6661c67f9f8a74005a4b944ac0`.
The consumer downloaded and visually inspected the package and selected original
`libfile_484da62ba9a881919f9256dd38b0f638` on 2026-10-02.

The package records AI-assisted extraction and monochrome conversion of the
selected artwork, followed by mechanical PNG sizing. This import performs no
redesign or resampling. The existing Xcode PNG catalog compiles the app icon;
no layered Icon Composer source is supplied or required by this pipeline.

| Current asset | Repository path | SHA-256 |
| --- | --- | --- |
| App 1024px | `apps/macos/AgentDeckApp/Assets.xcassets/AppIcon.appiconset/AppIcon-512@2x.png` | `c523c4c37131bf0cddb5b330ae757385595c4fa728ad2adb4b9be53775fb948e` |
| App runtime 512px | `apps/macos/AgentDeckApp/Resources/AgentDeckAppIcon.png` | `5cfa870ae21886da2e8aedb29ca0cde1933897b81642c47e5737688aaf12610f` |
| Template 18px | `apps/macos/AgentDeckApp/Assets.xcassets/AgentDeckMenuBarIcon.imageset/AgentDeckMenuBarIcon.png` | `0a7fe203e27cf1db01d9c1936b8487979cb826cb2871c36b21ed2664f2fce8a8` |
| Template runtime 36px | `apps/macos/AgentDeckApp/Resources/AgentDeckMenuBarIcon.png` | `47e3fa62dc13f9becfab61a7fded34a711237a442fb127346d20bfeb988c4255` |
| Active prototype symbol | `prototype/public/agentdeck-ad.png` | `2ca8d907a414f296bf09dd0f4a27c72f424016243efe04bb142a0120b6b6e6f1` |

App catalog sizes are the supplied 16/32/128/256/512pt 1x/2x derivatives.
Runtime PNG copies match their 512px app and 36px template sources. The menu
bar continues to render at 18pt with `isTemplate = true`; system appearance
owns its tint. Active prototype references use the supplied transparent symbol.
Historical prototypes and the original robot artwork remain unchanged.

Small-size limitation: the D cuts weaken at 16px. Static previews and technical
decode checks do not establish native display acceptance. See the [icon review record](../archive/topics/ad-shared-stroke-icon/reviews/ad-shared-stroke-icon.md)
for the checks actually performed.

### Rights boundary

This provenance is not a legal opinion, trademark clearance, uniqueness claim,
registration, or third-party license grant. The supplied limited similarity
screening did not complete visual inspection of Scalebranding #414379 because
its image was blocked. No trademark database search or global clearance is
claimed. The historical release-checkpoint guidance below remains applicable
to the current mark.

## Preserved historical record

The following records the former robot mark, not the current AD assets.
Paths and hashes below refer to the historical revision described there.

# Historical robot asset provenance

This document records the evidence chain for AgentDeck's current robot mark.
It is a provenance record, not a legal opinion, a trademark registration, or a
license grant to third parties.

## Current canonical assets

| Asset | Repository path | SHA-256 |
| --- | --- | --- |
| macOS App Icon source slot, 1024 px | `apps/macos/AgentDeckApp/Assets.xcassets/AppIcon.appiconset/AppIcon-512@2x.png` | `7af145fe92253efa67cfbfb11a4ef88034e3f4a11be15b7060bac9824137b95e` |
| macOS App Icon build resource, 512 px | `apps/macos/AgentDeckApp/Assets.xcassets/AppIcon.appiconset/AppIcon-512.png` | `fb1ad6434b818584d4c1b7bf984c230230cc778face9360c3d319eff9f2a9964` |
| Menu-bar template source, 36 px | `apps/macos/AgentDeckApp/Assets.xcassets/AgentDeckMenuBarIcon.imageset/AgentDeckMenuBarIcon@2x.png` | `0788eb64909ec049c129e12667b75233215bcba6522511e1df15e502c275f76a` |
| Menu-bar template source, 18 px | `apps/macos/AgentDeckApp/Assets.xcassets/AgentDeckMenuBarIcon.imageset/AgentDeckMenuBarIcon.png` | `00f7386fb45da012739d68445adc6bdd5e9ca0b594a1482adbe51b3b982d146b` |

The remaining App Icon slots are deterministic size derivatives. The copies in
`apps/macos/AgentDeckApp/Resources/` mirror the 512 px App Icon and 36 px
menu-bar assets used by the build.

## Generation and selection record

The current robot is not sourced from an identified third-party icon library.
Its recorded chain is:

1. On 2026-08-18, Codex session
   `01a014ca-57e4-7e30-8159-09882a6bc2a9` invoked OpenAI's built-in ImageGen
   tool to create AgentDeck macOS prototype boards from project screenshots and
   text instructions. The prompt prohibited copying CodeBurn branding or logo.
2. The generated prototype sequence produced the orange robot candidate. In the
   same session, the operator supplied a crop of that candidate as Image 3 and
   explicitly selected it for consistent menu-bar and popover identity. On
   2026-08-27, the operator separately confirmed that this image was generated
   by Codex rather than obtained from a third-party asset source.
3. A later ImageGen pass produced the selected v5 discussion board. The final
   board was stored during the session as `desktop-surfaces-v5-discussion.png`.
4. At session ordinal 3518, Codex extracted the selected mark with the recorded
   operation `crop=48:42:84:21,scale=120:105:flags=lanczos`, producing
   `agentdeck-robot.png`.
5. Git commit `f7a24f3d001d261f27cfb90825e23c8781b627bd` first preserved that
   prototype source; commit `5a76ce219c7d3e5edf7f9f7c117ed783d7588192` records it as blob
   `9e8d0f76e25db593482a6c941f34b6efbd91f3b8` at
   `docs/topics/desktop-app/ux/prototype/interactive-v7/public/agentdeck-robot.png`.
6. The app implementation removed only the exterior white matte, retained the
   enclosed white face, and generated the required App Icon sizes. A generative
   background-removal candidate was rejected because it redrew the robot and
   was not used. Commit `ce37a7a818e709b2fbbc4f8bff516d9a312370fa` first preserved the App
   Icon set, and `f37328dc077f7b5ab3b01d9d492ab971ab07a155` delivered it with the
   menu-bar application.
7. The current monochrome menu-bar template is a simplified derivative of the
   same selected robot silhouette. It does not introduce a separate third-party
   artwork source.

OpenAI's official
[Image Generation guide](https://platform.openai.com/docs/guides/image-generation)
documents that images can be created from text prompts or generated as part of
a conversation using the image-generation tool. That documentation supports
the technical generation mechanism recorded above; it does not by itself
establish copyrightability, uniqueness, or trademark clearance.

## Rights and use boundary

- AgentDeck treats the current mark as a first-party AI-generated brand asset
  selected and incorporated by the project operator.
- No recorded step identifies a stock-art, emoji, icon-library, or external
  brand asset as the robot source.
- This record does not guarantee that a generated output is unique or that no
  visually similar mark exists.
- Copyright protection for AI-assisted output can differ by jurisdiction. This
  record preserves human selection and the concrete derivative work, but makes
  no claim beyond rights available under applicable law.
- The word mark `AgentDeck` and the robot figurative mark have not been recorded
  here as registered trademarks or as having completed jurisdiction-specific
  clearance.
- Repository access or code reuse does not, through this document alone, grant
  permission to use the AgentDeck name or robot as a product mark.

## Commercial-release checkpoint

Before a commercial release materially relies on the mark, retain the session
and Git evidence above and perform a jurisdiction-appropriate word and
figurative-mark search for the intended software and hosted-service classes.
Record the search date, jurisdictions, classes, databases, and disposition in
this document or a linked legal review. A search-engine or reverse-image-search
miss is not trademark clearance.

Replace the mark through a clean-room design only if the recorded generation
chain is later contradicted, a materially similar earlier mark is found, or
formal counsel recommends replacement.

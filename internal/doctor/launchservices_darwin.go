//go:build darwin

package doctor

import (
	"context"
	"encoding/json"
	"sync"
)

const registrationScript = `ObjC.import('Foundation');
ObjC.bindFunction('access',['int',['char *','int']]);
ObjC.bindFunction('__error',['int *',[]]);
function run(argv) {
 var mode=argv[0], expected=mode==='host'?'com.kitdine.agentdeck':'com.kitdine.agentdeck.widget';
 var canonical=mode==='host'?'/Applications/AgentDeck.app':'/Applications/AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex';
 var left=262144, cache={};
 function bundle(path) {
  var e={path:path,canonical:false,state:'invalid_metadata'};
  try {
   if(Number($.access(path,0))!==0) {var code=Number($.__error()[0]);e.state=code===2?'missing':'unreadable_metadata';return e;}
   var attrs=ObjC.deepUnwrap($.NSFileManager.defaultManager.attributesOfItemAtPathError(path,null));
   if(!attrs) {e.state='unreadable_metadata';return e;}
   e.path=ObjC.unwrap($(path).stringByStandardizingPath.stringByResolvingSymlinksInPath);
   if(cache[e.path])return Object.assign({},cache[e.path]);
   if(left<=1) {e.state='unreadable_metadata';e.reason='output_limit';return e;}
   var h=$.NSFileHandle.fileHandleForReadingAtPath(e.path+'/Contents/Info.plist');
   if(!ObjC.unwrap(h)) {e.state='unreadable_metadata';return e;}
   var data;try {data=h.readDataOfLength(left);} finally {h.closeFile;}
   var n=Number(data.length);left-=n;
   if(left===0) {e.state='unreadable_metadata';e.reason='output_limit';return e;}
   var p=ObjC.deepUnwrap($.NSPropertyListSerialization.propertyListWithDataOptionsFormatError(data,0,null,null));
   if(!p || typeof p.CFBundleIdentifier!=='string' || typeof p.CFBundleShortVersionString!=='string' || !p.CFBundleShortVersionString || typeof p.CFBundleVersion!=='string'||!p.CFBundleVersion) return e;
   e.identifier=p.CFBundleIdentifier;e.version=p.CFBundleShortVersionString;e.build=p.CFBundleVersion;e.state='matching_build';cache[e.path]=e;return e;
  } catch(error) {return e;}
 }
 var installed=bundle(canonical), result={absent:false,complete:true,entries:[]};
 if(mode==='host' && installed.state==='missing') {result.absent=true;return JSON.stringify(result);}
 var paths=[];
 if(mode==='host') {
  try {
   ObjC.import('AppKit');var ws=$.NSWorkspace.sharedWorkspace;
   var control=ws.URLsForApplicationsWithBundleIdentifier('com.apple.finder'), count=Number(control.count), controlOK=false;
   if(!Number.isInteger(count)||count<=0||count>=64) {result.complete=false;result.reason='control_failed';}
   else for(var i=0;i<count;i++) {var b=bundle(ObjC.unwrap(control.objectAtIndex(i).path));if(b.identifier==='com.apple.finder'&&b.version&&b.build)controlOK=true;}
   if(!controlOK){result.complete=false;result.reason='control_failed';}
   var urls=ws.URLsForApplicationsWithBundleIdentifier(expected), n=Number(urls.count);
   if(!Number.isInteger(n)||n<0){result.complete=false;result.reason=result.reason||'enumeration_failed';}
   else {var returned={};for(var j=0;j<n;j++) {var raw=ObjC.unwrap(urls.objectAtIndex(j).path), normalized=ObjC.unwrap($(raw).stringByStandardizingPath.stringByResolvingSymlinksInPath);if(returned[normalized])continue;returned[normalized]=true;paths.push(raw);if(paths.length>=64){result.complete=false;result.reason=result.reason||'entry_limit';break;}}}
  }catch(error){result.complete=false;result.reason=result.reason||'enumeration_failed';}
 } else {paths=JSON.parse(argv[1]);}
 var seen={};
 for(var k=0;k<paths.length;k++) {
  if(typeof paths[k]!=='string'||paths[k][0]!=='/') {result.complete=false;result.reason=result.reason||'unknown_format';continue;}
  var e=bundle(paths[k]);if(seen[e.path])continue;seen[e.path]=true;
  if(e.identifier!==undefined && e.identifier!==expected)e.state='invalid_metadata';
  e.canonical=e.path===installed.path && installed.state==='matching_build' && installed.identifier===expected && e.state==='matching_build';
  if(e.canonical)e.state='canonical';
  if(e.state==='invalid_metadata'||e.state==='unreadable_metadata'){result.complete=false;result.reason=result.reason||e.reason||e.state;}
  delete e.identifier;delete e.reason;result.entries.push(e);
  if(result.entries.length>=64){result.complete=false;result.reason=result.reason||'entry_limit';break;}
 }
 if(installed.state!=='matching_build' || installed.identifier!==expected){result.complete=false;result.reason=result.reason||'canonical_missing';}
 return JSON.stringify(result);
}`

type registrationNative struct {
	Absent   bool                `json:"absent"`
	Complete bool                `json:"complete"`
	Reason   string              `json:"reason,omitempty"`
	Entries  []RegistrationEntry `json:"entries"`
}

func nativeRegistrationSource(ctx context.Context, mode string, paths []string) (RegistrationSource, bool) {
	return nativeRegistrationSourceLimited(ctx, mode, paths, registrationOutputLimit)
}

func nativeRegistrationSourceLimited(ctx context.Context, mode string, paths []string, outputAllowance int) (RegistrationSource, bool) {
	s := RegistrationSource{Source: "host_application_urls", Entries: []RegistrationEntry{}}
	if mode != "host" {
		s.Source = "widget_pluginkit"
	}
	data, _ := json.Marshal(paths)
	output, reason := registrationCommandLimited(ctx, outputAllowance, "/usr/bin/osascript", "-l", "JavaScript", "-e", registrationScript, mode, string(data))
	if reason != "" {
		sourceFailure(&s, reason)
		return s, false
	}
	var result registrationNative
	if !decodeRegistration(output, &result) {
		sourceFailure(&s, "unknown_format")
		return s, false
	}
	s.Complete = result.Complete
	s.Reason = result.Reason
	if s.Reason != "" {
		s.Complete = false
	}
	s.Entries = result.Entries
	return s, result.Absent
}

func acquireRegistration(ctx context.Context) RegistrationDetails {
	d := RegistrationDetails{Applicable: true, Host: RegistrationSource{Source: "host_application_urls", Entries: []RegistrationEntry{}}, Widget: RegistrationSource{Source: "widget_pluginkit", Entries: []RegistrationEntry{}}}
	var wg sync.WaitGroup
	var absent bool
	wg.Add(2)
	go func() { defer wg.Done(); d.Host, absent = nativeRegistrationSource(ctx, "host", nil) }()
	go func() {
		defer wg.Done()
		output, reason := registrationCommand(ctx, "/usr/bin/pluginkit", "-m", "-A", "-D", "-vv", "-i", "com.kitdine.agentdeck.widget")
		if reason != "" {
			sourceFailure(&d.Widget, reason)
			return
		}
		d.Widget = widgetSourceFromOutput(ctx, output, func(ctx context.Context, paths []string, allowance int) RegistrationSource {
			src, _ := nativeRegistrationSourceLimited(ctx, "widget", paths, allowance)
			return src
		})
	}()
	wg.Wait()
	if absent {
		d.Applicable = false
		d.ApplicabilityReason = "gui_host_absent"
	}
	return d
}

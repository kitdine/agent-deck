(async () => {
  const fail=[];let checks=0;
  const tick=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
  const test=(name,ok)=>{checks++;if(!ok)fail.push(name);};
  const stage=document.querySelector('select');
  const scene=async key=>{stage.value=key;stage.dispatchEvent(new Event('change',{bubbles:true}));await tick();};
  const scope=async n=>{document.querySelectorAll('.asc-scope button')[n].click();await tick();};
  const val=selector=>document.querySelector(selector)?.textContent;
  const geometry=()=>Array.from(document.querySelectorAll('.asc-panel *')).filter(x=>x.clientWidth&&x.scrollWidth>x.clientWidth+1).length===0;
  const hero=document.querySelector('.hero').textContent;
  for(const state of ['normal','empty','unknown','partial','stale','loading','failed','unavailable','override']) {
    await scene(state);
    for(let n=0;n<3;n++) {await scope(n);test(`${state}/${n} geometry`,geometry());test(`${state}/${n} consumption unchanged`,document.querySelector('.hero').textContent===hero);
      if(state==='unavailable')test('unavailable has no count',!document.querySelector('[data-asc-count]'));
      else if(['loading','failed'].includes(state))test(`${state} retains Work`,val('[data-asc-count]')==='3'&&document.querySelector('.asc-panel').dataset.ascScope==='work');
      else {const expected=state==='empty'?[0,0,0]:state==='unknown'?[4,0,4]:[3,1,4];test(`${state}/${n} count`,val('[data-asc-count]')===String(expected[n]));}
    }
  }
  await scene('normal');
  const expected=[[[3,1,4],[4,1,5],[4,2,6]],[[2,0,2],[2,0,2],[2,1,3]],[[1,1,2],[2,1,3],[2,1,3]]];
  for(let c=0;c<3;c++)for(let p=0;p<3;p++)for(let s=0;s<3;s++){
    document.querySelectorAll('.segmented.clients button')[c].click();document.querySelectorAll('.segmented.periods button')[p].click();await tick();await scope(s);
    test(`client ${c}/period ${p}/scope ${s}`,val('[data-asc-count]')===String(expected[c][p][s]));
  }
  document.querySelectorAll('.segmented.clients button')[0].click();document.querySelectorAll('.segmented.periods button')[0].click();await tick();await scope(0);
  document.querySelector('[data-asc-id="old-01"]').click();await tick();test('detail focuses back',document.activeElement===document.querySelector('.asc-panel > button'));
  document.activeElement.click();await tick();test('back restores row focus',document.activeElement?.dataset.ascId==='old-01');
  document.querySelector('[data-asc-id="child-1"]').click();await tick();const childFields=document.querySelectorAll('.asc-detail dd');test('task child remains in Work',document.querySelector('.asc-panel').dataset.ascScope==='work');test('task child automatic and effective unknown',Array.from(childFields).slice(0,2).every(x=>/Unknown|未知/.test(x.textContent)));test('task child is not a user override',childFields[4].textContent==='0'&&!/user override|用户覆盖/.test(childFields[0].textContent));test('task child reason reachable',/Task-agent fixture|任务子代理样例/.test(childFields[2].textContent));
  document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));await tick();test('Escape returns',!!document.querySelector('[data-asc-count]'));
  await scope(1);document.querySelector('[data-tab="usage"]').click();await tick();document.querySelector('[data-tab="sessions"]').click();await tick();test('scope survives tab',val('[data-asc-count]')==='1');
  document.querySelector('.menubar-item').click();await tick();document.querySelector('.menubar-item').click();await tick();test('reopen restores default',document.querySelector('.asc-panel')?.dataset.ascScope==='work');
  await scene('failed');document.querySelector('.asc-panel > button').click();await tick();test('retry publishes requested scope',document.querySelector('.asc-panel').dataset.ascScope==='auxiliary'&&val('[data-asc-count]')==='1');
  const recovered=(label,count)=>{
    const panel=document.querySelector('.asc-panel');
    test(`${label}: normal auxiliary selection`,panel.dataset.ascState==='normal'&&panel.dataset.ascScope==='auxiliary');
    test(`${label}: count and controls`,val('[data-asc-count]')===String(count)&&Array.from(panel.querySelectorAll('.asc-scope button')).every(b=>!b.disabled));
    test(`${label}: no stale failure notice`,!panel.querySelector('.asc-notice').textContent.match(/could not load|读取失败|previous Work|保留工作/));
  };
  document.querySelector('[data-tab="usage"]').click();await tick();test('retry roundtrip unmounts panel',!document.querySelector('.asc-panel'));
  document.querySelector('[data-tab="sessions"]').click();await tick();recovered('retry tab roundtrip',1);
  test('retry roundtrip retains auxiliary record',!!document.querySelector('[data-asc-id="maint-1"]')&&!document.querySelector('[data-asc-id="task-01"]'));
  for(let c=0;c<3;c++)for(let p=0;p<3;p++){
    document.querySelectorAll('.segmented.clients button')[c].click();document.querySelectorAll('.segmented.periods button')[p].click();await tick();
    recovered(`retry client ${c}/period ${p}`,expected[c][p][1]);
  }
  document.querySelector('.menubar-item').click();await tick();document.querySelector('.menubar-item').click();await tick();
  test('new failed fixture reopens consistently',document.querySelector('.asc-panel').dataset.ascState==='failed'&&document.querySelector('.asc-panel').dataset.ascScope==='work'&&val('[data-asc-count]')==='3');
  await scene('normal');await scope(1);
  document.querySelector('.menubar-item').click();await tick();document.querySelector('.menubar-item').click();await tick();
  test('normal reopen restores default after recovery',document.querySelector('.asc-panel').dataset.ascState==='normal'&&document.querySelector('.asc-panel').dataset.ascScope==='work');
  for(const state of ['legacy','defaultUnknown']){
    await scene(state);test(state+' has no classified counts',!document.querySelector('[data-asc-count]'));test(state+' has no guessed scope controls',!document.querySelector('.asc-scope'));
    document.querySelector('[data-tab="usage"]').click();await tick();document.querySelector('[data-tab="sessions"]').click();await tick();test(state+' survives tab',!document.querySelector('[data-asc-count]'));
  }
  for(const state of ['sourceUnavailable','inputMismatch']){
    await scene(state);
    for(let s=0;s<3;s++){await scope(s);test(state+'/'+s+' conservative membership',val('[data-asc-count]')===String([4,0,4][s]));test(state+'/'+s+' geometry',geometry());}
    await scope(0);document.querySelector('[data-asc-id="maint-1"]').click();await tick();
    const fields=document.querySelectorAll('.asc-detail dd');test(state+' automatic remains unknown',/Unknown|未知/.test(fields[1].textContent));
    if(state==='sourceUnavailable')test('missing rule unavailable',/Unavailable|不可用/.test(fields[3].textContent));
    let copied='';Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async value=>{copied=value;}}});
    document.querySelector('.asc-detail button').click();await tick();test('copy explicit ID and quoted arguments',copied==="agentdeck session classify --id 'maint-1' --client 'claude'");
  }
  await scene('normal');document.querySelectorAll('.segmented.clients button')[2].click();await tick();await scope(0);test('all missing durations not zero',val('[data-asc-minutes]')==='—');
  document.querySelectorAll('.segmented.clients button')[0].click();await tick();
  return {checks,failures:fail,lang:document.querySelector('main').dataset.theme};
})()

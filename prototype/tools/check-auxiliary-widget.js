(async()=>{
 const failures=[];let checks=0;const tick=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
 const test=(n,v)=>{checks++;if(!v)failures.push(n);};
 const select=async(s,v)=>{const el=document.querySelector(s);el.value=v;el.dispatchEvent(new Event('change',{bubbles:true}));await tick();};
 const countText=()=>Array.from(document.querySelectorAll('[data-aw-count]')).map(x=>x.textContent);
 const prices=()=>Array.from(document.querySelectorAll('.w-headline,.w-hero strong,.w-periods strong')).map(x=>x.textContent).join('|');const before=prices();
 for(const state of ['normal','empty','unknown','partial','legacy','defaultUnknown','pending','stale'])for(const scope of ['work','auxiliary','all']){
  await select('[data-aw-state]',state);await select('[data-aw-scope-select]',scope);
  test(`${state}/${scope} consumption unchanged`,prices()===before);
  const nodes=Array.from(document.querySelectorAll('[data-aw-count]'));
  test(`${state}/${scope} existing count surfaces only`,nodes.length===2&&!document.querySelector('.widget-medium [data-aw-count]'));
  for(const node of nodes){const today=node.dataset.awPeriod==='today';const unavailable=['legacy','defaultUnknown'].includes(state);
   if(unavailable)test(`${state}/${scope} unavailable`,node.dataset.awScope==='unavailable'&&!/\d/.test(node.textContent));
   else{const expected=state==='empty'?0:state==='unknown'?(scope==='auxiliary'?0:today?4:6):({work:today?3:4,auxiliary:today?1:2,all:today?4:6})[scope];test(`${state}/${scope}/${node.dataset.awPeriod} count`,node.dataset.awScope===scope&&new RegExp(` ${expected} (会话|sessions) · `).test(node.textContent));test(`${state}/${scope} unknown`,state!=='partial'||scope==='auxiliary'||node.textContent.endsWith('—'));}
  }
  for(const card of document.querySelectorAll('.widget'))test(`${state}/${scope}/${card.className} bounds`,Array.from(card.querySelectorAll('*')).every(x=>!x.clientWidth||(x.scrollWidth<=x.clientWidth+1&&x.scrollHeight<=x.clientHeight+1)));
  const shown=countText();await select('[data-aw-default]',scope==='work'?'auxiliary':'work');test(`${state}/${scope} default cannot relabel snapshot`,JSON.stringify(countText())===JSON.stringify(shown));
 }
 test('small seven buckets',document.querySelectorAll('.widget-small .mini-bars i').length===7);
 test('medium twenty buckets',document.querySelectorAll('.widget-medium .mini-bars i').length===20);
 return {checks,failures};
})()

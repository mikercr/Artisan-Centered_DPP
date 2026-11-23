'use strict';

// CraftPass Studio: zero-dependency web client. Authentication credentials are
// kept in volatile JS memory (never persisted to localStorage or a cookie).
const app = { view: 'overview', meta: null, actor: '', token: '', lists: {}, passport: null, busy: false };
const $ = (id) => document.getElementById(id);
const safe = (value) => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const pretty = (data) => JSON.stringify(data, null, 2);
const titleCase = (str) => str.charAt(0).toUpperCase() + str.slice(1);
const slug = (str) => String(str || '').replace(/[^A-Za-z0-9._:-]/g, '');

async function request(method, url, payload) {
  const headers = {};
  if (app.token) headers.Authorization = `Bearer ${app.token}`;
  if (app.actor) headers['X-Actor'] = app.actor;
  if (payload !== undefined) headers['Content-Type'] = 'application/json';
  const response = await fetch(url, {method, headers, body: payload === undefined ? undefined : JSON.stringify(payload), cache: 'no-store'});
  let data;
  try { data = await response.json(); } catch (_) { throw Error(`Server returned HTTP ${response.status} without JSON`); }
  if (!response.ok) throw Error(data.error || `HTTP ${response.status}`);
  return data.data ?? data;
}
function toast(message, error=false) {
  const el = $('toast'); el.textContent = message; el.className = 'toast show' + (error ? ' error' : '');
  clearTimeout(toast.timer); toast.timer = setTimeout(() => el.className='toast', 5500);
}
function showError(error) { toast(error.message || String(error), true); }
function notice(text, warning=false) { return `<div class="notice${warning?' warn':''}">${safe(text)}</div>`; }
function pageHeading(label, heading, description, action='') {
  return `<div class="page-header"><div><div class="eyebrow">${safe(label)}</div><h1>${safe(heading)}</h1><p>${safe(description)}</p></div>${action}</div>`;
}
function metric(label, value, text) { return `<article class="metric"><span class="kicker">${safe(label)}</span><strong class="number">${safe(value)}</strong><span class="mini">${safe(text)}</span></article>`; }
function field(name, label, opts={}) {
  const required = opts.required ? ' required' : '';
  const hint = opts.hint ? `<small>${safe(opts.hint)}</small>` : '';
  let widget;
  if (opts.type === 'checkbox') {
    return `<label class="field checkbox"><input type="checkbox" name="${safe(name)}"${opts.checked?' checked':''}><span>${safe(label)}</span></label>`;
  }
  if (opts.type === 'select') {
    widget = `<select name="${safe(name)}"${required}>${(opts.options||[]).map(v=>`<option value="${safe(v.value??v)}">${safe(v.label??v)}</option>`).join('')}</select>`;
  } else if (opts.type === 'textarea') {
    widget = `<textarea name="${safe(name)}" placeholder="${safe(opts.placeholder||'')}"${required}></textarea>`;
  } else {
    widget = `<input name="${safe(name)}" type="${safe(opts.type||'text')}" placeholder="${safe(opts.placeholder||'')}" ${opts.min!==undefined ? `min="${opts.min}"` : ''}${required}>`;
  }
  return `<label class="field"><span>${safe(label)}${opts.required?' *':''}</span>${widget}${hint}</label>`;
}
function getFormData(form) {
  const values = {};
  new FormData(form).forEach((value, key) => values[key] = String(value).trim());
  return values;
}
function requiredInt(value, label) {
  const x = Number(value);
  if (!Number.isSafeInteger(x) || x <= 0) throw Error(`${label} must be a positive integer (grams)`);
  return x;
}
function collectionMarkup(name, headers, fields, addLabel) {
  const template = fields.map((f,i)=>`<input name="${safe(name)}_${i}" ${f.type?'type="'+safe(f.type)+'"':''} ${f.type==='number'?'min="1"':''} placeholder="${safe(f.placeholder||headers[i])}" aria-label="${safe(headers[i])}" ${f.required?'required':''}>`).join('');
  return `<div class="field"><span>${safe(addLabel)}</span><div class="collection" data-collection="${safe(name)}" data-fields="${fields.length}" data-template="${safe(template)}"></div><button type="button" class="secondary-button add-row" data-add="${safe(name)}">+ Add ${safe(name.slice(0,-1)||name)}</button></div>`;
}
function addCollectionRow(container) {
  const row = document.createElement('div'); row.className = 'collection-row';
  // The template originates exclusively in the static form definition.
  row.innerHTML = container.dataset.template + '<button type="button" data-remove aria-label="Remove row">×</button>';
  container.appendChild(row);
}
function extractCollection(form, name, keys, numeric=[]) {
  return [...form.querySelectorAll(`[data-collection="${name}"] .collection-row`)].map(row => {
    const obj = {};
    row.querySelectorAll('input').forEach((input,i) => {
      const value = input.value.trim();
      if (!value) throw Error(`${keys[i]} is required for every ${name} entry`);
      obj[keys[i]] = numeric.includes(keys[i]) ? requiredInt(value, keys[i]) : value;
    });
    return obj;
  });
}

const screens = {
  overview: () => {
    const lists = app.lists;
    const products = lists.products || [];
    const published = products.filter(p=>p.status==='published').length;
    const metrics = [metric('Registered artisans',(lists.artisans||[]).length,'Skills and contributors'),metric('Material lots',(lists.materials||[]).length,'Origins and custody'),metric('Craft products',products.length,'Tracked individually'),metric('Published DPPs',published,'Publicly retrievable')].join('');
    const recent = products.slice(-5).reverse();
    return pageHeading('PROVENANCE INTELLIGENCE','Every object has a story.','A shared workspace for linking handcrafted products to the materials, people and techniques behind them.',`<button class="primary-button" data-go="products" data-form="1">+ New product</button>`) +
      (app.meta.mode==='fabric'?notice('Fabric mode: record enumeration is unavailable in this chaincode. Use record IDs to retrieve individual passports and resources.',true):'')+
      `<div class="metric-grid">${metrics}</div><div class="two-col"><section class="card"><div class="section-head"><h2>Recent products</h2><button class="text-button" data-go="products">All products →</button></div>${recent.length?recent.map(p=>`<div class="item-row"><div><strong>${safe(p.name)}</strong><small>${safe(p.id)} · ${safe(p.category)}</small></div><button class="text-button" data-passport="${safe(p.id)}">View passport →</button></div>`).join(''):`<div class="empty">Create a product to begin building its provenance.</div>`}</section>
      <section class="card"><h2>Traceability workflow</h2><div class="step-guide"><div class="item-row"><span class="tag">01</span><div><strong>Register source materials</strong><small>Track origin, quantity and custody.</small></div></div><div class="item-row"><span class="tag">02</span><div><strong>Associate skilled artisans</strong><small>Preserve contributor roles and techniques.</small></div></div><div class="item-row"><span class="tag">03</span><div><strong>Record production activities</strong><small>Link each stage to materials and people.</small></div></div><div class="item-row"><span class="tag">04</span><div><strong>Publish and attest</strong><small>Present the DPP and audit claims.</small></div></div></div></section></div>`;
  },
  passports: () => pageHeading('DIGITAL PRODUCT PASSPORTS','Explore provenance','Trace a product from materials and techniques to the artisans who shaped it.')+
    `<div class="card"><h2>Find a passport</h2><p class="subtle">Passports can be read publicly by product ID. Sample: <button class="text-button" data-passport="vase-001">vase-001</button></p><form id="passport-search" class="actionbar"><input name="id" placeholder="Enter product ID (e.g. vase-001)" required><button class="primary-button" type="submit">View passport →</button></form></div><div id="passport-result" class="passport-panel"></div>`,
  products: () => screenRecords('products', 'Production catalogue', 'Manage individual handcrafted products and publish their product passports.', 'CreateProduct'),
  artisans: () => screenRecords('artisans', 'The people behind the craft', 'Register contributors, skills, and privacy-safe public aliases.', 'RegisterArtisan'),
  materials: () => screenRecords('materials', 'Material provenance', 'Register source lots, follow custody and document derived batches.', 'RegisterMaterial'),
  activities: () => screenRecords('activities', 'Production timeline', 'Connect each manufacturing stage with skilled artisans, techniques and material lots.', 'RecordActivity'),
  attestations: () => pageHeading('INDEPENDENT ASSURANCE','Audit attestations','Record an independent auditor’s supported or disputed assertion for a production activity.')+formSection('AttestActivity') + `<div class="card" style="margin-top:20px"><h2>Verify activity evidence</h2><p class="subtle">Check that a submitted hash matches the activity’s recorded evidence digest. This checks hash equality, not authenticity or authorship.</p><form id="verify-form" class="actionbar"><input name="activityId" placeholder="Activity ID" required><input name="sha256" placeholder="64-character lowercase SHA-256 hex" required minlength="64" maxlength="64"><button class="secondary-button">Verify digest</button></form><div id="verification-result"></div></div>`
};
function screenRecords(kind, heading, desc, method) {
  const listing = app.lists[kind];
  return pageHeading('PROVENANCE REGISTRY',heading,desc,`<button class="primary-button" data-toggle-form>+ New ${kind.slice(0,-1)}</button>`) +
    `<div id="resource-form" hidden>${formSection(method)}</div>` +
    (kind==='materials'?`<div class="card" style="margin-bottom:20px"><div class="section-head"><h2>Custody and transformations</h2></div><div class="form-actions"><button class="secondary-button" data-special="transfer">Transfer material</button><button class="secondary-button" data-special="derive">Derive material lot</button></div><div id="special-form" style="margin-top:20px"></div></div>`:'')+
    `<div class="surface"><div class="card"><div class="section-head"><h2>Recorded ${safe(kind)}</h2><span class="tag">${listing?listing.length+' records':'Lookup by ID'}</span></div>${listing?recordTable(kind,listing):`<form class="actionbar" data-lookup="${safe(kind)}"><input name="id" placeholder="Enter ${safe(kind.slice(0,-1))} ID" required><button class="secondary-button">Find record</button></form><div id="lookup-result"></div>`}</div></div>`;
}
function recordTable(kind, list) {
  if (!list.length) return `<div class="empty"><span class="symbol">◇</span>No ${safe(kind)} found yet.</div>`;
  const fields = ({products:['id','name','category','status'], artisans:['id','publicAlias','skills','public'], materials:['id','materialType','originRegion','quantityGrams','custodianMSP'], activities:['id','productId','stage','technique']})[kind]||['id'];
  return `<div class="table-scroll"><table class="records-table"><thead><tr>${fields.map(f=>`<th>${safe(f.replace(/([A-Z])/g,' $1'))}</th>`).join('')}<th>Action</th></tr></thead><tbody>${list.map(obj=>`<tr>${fields.map(f=>`<td>${f==='status'?`<span class="pill">${safe(obj[f]||'')}</span>`:safe(Array.isArray(obj[f])?obj[f].join(', '):obj[f])}</td>`).join('')}<td><button class="text-button" data-record="${safe(kind)}" data-id="${safe(obj.id)}">View →</button></td></tr>`).join('')}</tbody></table></div>`;
}
function formSection(method) {
  const main={
    RegisterArtisan: {title:'Register an artisan',fields:[field('id','Artisan ID',{required:true,placeholder:'artisan-maria'}),field('publicAlias','Display alias',{placeholder:'Maria Costa'}),field('skills','Skills (comma-separated)',{placeholder:'Pottery, Wheel throwing'}),field('public','Show alias in public passports',{type:'checkbox',checked:true})]},
    RegisterMaterial:{title:'Register a material lot',fields:[field('id','Lot ID',{required:true,placeholder:'clay-2026-01'}),field('materialType','Material type',{required:true,placeholder:'Red clay'}),field('originRegion','Origin region',{required:true,placeholder:'Alentejo, Portugal'}),field('quantityGrams','Quantity in grams',{required:true,type:'number',min:1}),field('supplierRef','Supplier reference'),field('evidenceSha256','Evidence digest (SHA-256)',{hint:'Optional 64-digit lowercase hex digest; evidence stays off-chain.'})]},
    CreateProduct:{title:'Create a product',fields:[field('id','Product ID',{required:true,placeholder:'vase-023'}),field('name','Product name',{required:true,placeholder:'Handcrafted ceramic vase'}),field('category','Category',{required:true,placeholder:'Ceramics'})]},
    RecordActivity:{title:'Record a production activity',fields:[field('id','Activity ID',{required:true,placeholder:'vase-023-shaping'}),field('productId','Product ID',{required:true,placeholder:'vase-023'}),field('stage','Production stage',{required:true,placeholder:'Shaping'}),field('technique','Technique',{required:true,placeholder:'Wheel throwing'}),field('locationCode','Workshop / location code'),field('evidenceSha256','Evidence digest (SHA-256)'),collectionMarkup('contributors',['Artisan ID','Role'],[{placeholder:'artisan-maria'},{placeholder:'Potter'}],'Artisan contributions'),collectionMarkup('materials',['Material lot ID','Grams'],[{placeholder:'clay-2026-01'},{placeholder:'850',type:'number'}],'Material allocations')]},
    TransferMaterial:{title:'Transfer material custody',fields:[field('lotId','Material lot ID',{required:true,placeholder:'clay-2026-01'}),field('toMSP','Destination MSP',{required:true,placeholder:'WorkshopMSP'})]},
    DeriveMaterial:{title:'Derive a material lot',fields:[field('id','New lot ID',{required:true}),field('materialType','Output material type',{required:true}),field('quantityGrams','Output quantity in grams',{required:true,type:'number',min:1}),field('evidenceSha256','Evidence digest (SHA-256)'),collectionMarkup('sourceLots',['Source lot ID','Consumed grams'],[{placeholder:'source-lot-id'},{type:'number',placeholder:'1000'}],'Source materials')]},
    AttestActivity:{title:'Submit independent attestation',fields:[field('activityId','Activity ID',{required:true}),field('verdict','Verdict',{type:'select',options:['supported','disputed']}),field('evidenceSha256','Evidence digest (SHA-256)',{required:true,hint:'A SHA-256 hex digest of the auditor’s off-chain evidence.'})]}
  }[method];
  return `<section class="form-card"><h2>${safe(main.title)}</h2><form data-method="${method}"><div class="field-grid">${main.fields.join('')}</div><div class="form-actions"><button type="submit" class="primary-button">Submit transaction</button><span class="subtle">A certificate’s role controls Fabric permissions.</span></div></form></section>`;
}
function getPayload(method, form) {
  const raw = getFormData(form);
  if (method==='RegisterArtisan') return {id:raw.id, publicAlias:raw.publicAlias||'', skills:(raw.skills||'').split(',').map(x=>x.trim()).filter(Boolean), public:form.elements.public.checked};
  if (method==='RegisterMaterial') return { ...raw, quantityGrams:requiredInt(raw.quantityGrams,'Quantity') };
  if (method==='CreateProduct') return raw;
  if (method==='RecordActivity') return {...raw,contributors:extractCollection(form,'contributors',['artisanId','role']),materials:extractCollection(form,'materials',['lotId','quantityGrams'],['quantityGrams'])};
  if (method==='DeriveMaterial') return {...raw,quantityGrams:requiredInt(raw.quantityGrams,'Output quantity'),sourceLots:extractCollection(form,'sourceLots',['lotId','quantityGrams'],['quantityGrams'])};
  return raw;
}
async function submitTransaction(form) {
  const method = form.dataset.method, payload=getPayload(method,form);
  let url;
  if (method==='RegisterArtisan')url='/api/artisans';
  else if (method==='RegisterMaterial')url='/api/materials';
  else if (method==='CreateProduct')url='/api/products';
  else if (method==='RecordActivity')url='/api/activities';
  else if (method==='DeriveMaterial')url='/api/materials/derive';
  else if (method==='AttestActivity')url='/api/attestations';
  else if (method==='TransferMaterial')url=`/api/materials/${encodeURIComponent(payload.lotId)}/transfer`;
  if (method==='TransferMaterial') { delete payload.lotId; }
  const result=await request('POST',url,payload);
  toast(`Transaction committed: ${result.id||result.activityId||method}`);
  if(app.meta.mode==='demo')await loadCatalog();
  render();
}
async function loadCatalog() {
  if (!app.meta || app.meta.mode !== 'demo') { app.lists={}; return; }
  const entries=await Promise.all(['artisans','materials','products','activities'].map(async name=>[name,await request('GET',`/api/records/${name}`)]));
  app.lists=Object.fromEntries(entries);
}
async function openPassport(id) {
  if (!id) return;
  try { const passport=await request('GET',`/api/passports/${encodeURIComponent(id)}`);app.passport=passport;
    showView('passports');
    $('passport-result').innerHTML=renderPassport(passport);
    const input=$('passport-search').elements.id; input.value=id;
  } catch(error) {showError(error);}
}
function renderPassport(passport) {
  const acts=passport.activities||[], lots=passport.materials||[];
  const header=`<div class="passport-header"><div class="eyebrow">DIGITAL PRODUCT PASSPORT</div><h2>${safe(passport.name)}</h2><p>${safe(passport.category)} · Item reference: <span class="mono">${safe(passport.id)}</span></p><div class="passport-meta"><span class="pill">${safe(passport.status)}</span><span>${acts.length} production stages</span><span>${lots.length} material lots</span></div></div>`;
  const timeline=acts.length?acts.map(a=>`<div class="step"><h3>${safe(a.stage)}</h3><p>${safe(a.technique)}${a.locationCode?' · '+safe(a.locationCode):''}</p><div class="chips">${(a.artisans||[]).map(x=>`<span class="chip">${safe(x.alias||x.id)} · ${safe(x.role)}</span>`).join('')}${(a.materials||[]).map(x=>`<span class="chip">${safe(x.lotId)} · ${safe(x.quantityGrams)}g</span>`).join('')}${(a.attestations||[]).map(x=>`<span class="chip">Audit: ${safe(x.verdict)} (${safe(x.auditorMSP)})</span>`).join('')}</div></div>`).join(''):`<div class="empty">No production activities recorded.</div>`;
  const materials=lots.length?lots.map(m=>`<div class="item-row"><div><strong>${safe(m.materialType)}</strong><small>${safe(m.id)} · ${safe(m.originRegion)}</small>${(m.sourceLots||[]).length?`<small>Derived from ${m.sourceLots.map(l=>safe(l.lotId)).join(', ')}</small>`:''}</div></div>`).join(''):`<div class="empty">No source materials linked.</div>`;
  return `${header}<div class="two-col passport-panel"><section class="card"><h2>Production journey</h2><div class="timeline">${timeline}</div></section><section class="card"><h2>Material origins</h2>${materials}<div class="divider"></div><h2>Passport integrity</h2><div class="notice warn">Recorded claims and hash matches do not automatically verify real-world authenticity. Evidence documents are stored off-chain.</div><button class="secondary-button" id="toggle-json">Show machine-readable passport JSON</button><pre id="raw-json" class="json-panel" hidden></pre></section></div>`;
}
async function lookupRecord(kind,id) {
  const route=({artisans:'artisans',materials:'materials',products:'products',activities:'activities'})[kind];
  const result=await request('GET',`/api/${route}/${encodeURIComponent(id)}`);
  if (kind==='products'){ await openPassport(id);return; }
  const content=`<div class="card" style="margin-top:16px"><div class="section-head"><h2>Record: ${safe(id)}</h2><button class="text-button" data-dismiss-record>Close ×</button></div><pre class="json-panel">${safe(pretty(result))}</pre></div>`;
  const el=$('lookup-result');
  if (el) el.innerHTML=content;
  else { const panel=document.createElement('div'); panel.id='lookup-result'; panel.innerHTML=content; document.querySelector('.surface')?.after(panel); }
}
function showView(view) {
  app.view=screens[view]?view:'overview';
  window.history.replaceState(null,'','#'+app.view);
  document.querySelectorAll('.nav-link').forEach(b=>b.classList.toggle('active',b.dataset.view===app.view));
  $('location-title').textContent=titleCase(app.view);
  render();
}
function render(){ $('main-content').innerHTML=screens[app.view](); }
async function initialize() {
  try {
    app.meta=await request('GET','/api/meta');
    $('status-dot').classList.add('online'); $('connection-status').textContent='API connected';
    $('mode-badge').textContent=app.meta.mode==='demo'?'● Demo ledger':'● Hyperledger Fabric';
    const actors=app.meta.actors||[];
    $('actor-select').innerHTML=actors.map(a=>`<option value="${safe(a.id)}">${safe(a.label)} · ${safe(a.role)}</option>`).join('');
    app.actor=actors.find(a=>a.role==='producer')?.id||actors[0]?.id||'';
    $('actor-select').value=app.actor;
    try {await loadCatalog();}catch(err) {showError(err);}
    showView(location.hash.slice(1)||'overview');
  } catch(err) {
    $('status-dot').classList.remove('online');$('connection-status').textContent='API unreachable';
    $('main-content').innerHTML=pageHeading('CONNECTION ERROR','Unable to connect','Start the Go server and reload this page.')+notice(err.message,true);
  }
}

document.addEventListener('click', async e=>{
  const b=e.target.closest('button');if(!b)return;
  if(b.dataset.view)showView(b.dataset.view);
  if(b.dataset.go){ showView(b.dataset.go);if(b.dataset.form){const panel=$('resource-form');if(panel)panel.hidden=false;} }
  if(b.hasAttribute('data-toggle-form')){const panel=$('resource-form');panel.hidden=!panel.hidden;}
  if(b.dataset.special){$('special-form').innerHTML=formSection(b.dataset.special==='transfer'?'TransferMaterial':'DeriveMaterial');}
  if(b.dataset.add){const c=document.querySelector(`[data-collection="${b.dataset.add}"]`);if(c)addCollectionRow(c);}
  if(b.hasAttribute('data-remove'))b.closest('.collection-row').remove();
  if(b.dataset.passport)await openPassport(b.dataset.passport);
  if(b.dataset.record)try{await lookupRecord(b.dataset.record,b.dataset.id);}catch(err){showError(err);}
  if(b.hasAttribute('data-dismiss-record'))$('lookup-result')?.remove();
  if(b.id==='toggle-json'&&app.passport){const el=$('raw-json');el.hidden=!el.hidden;el.textContent=pretty(app.passport);b.textContent=el.hidden?'Show machine-readable passport JSON':'Hide machine-readable JSON';}
});
document.addEventListener('submit',async e=>{
  const form=e.target;
  if(form.matches('form[data-method]')) {e.preventDefault();const btn=form.querySelector('[type=submit]');btn.disabled=true;try{await submitTransaction(form);}catch(err){showError(err);}finally{btn.disabled=false;} }
  if(form.id==='passport-search'){e.preventDefault();await openPassport(form.elements.id.value.trim());}
  if(form.dataset.lookup){e.preventDefault();try{await lookupRecord(form.dataset.lookup,form.elements.id.value.trim());}catch(err){showError(err);}}
  if(form.id==='verify-form'){e.preventDefault();try{const p=getFormData(form);const result=await request('GET',`/api/activities/${encodeURIComponent(p.activityId)}/evidence/${encodeURIComponent(p.sha256)}`);$('verification-result').innerHTML=notice(result===true?'Evidence digest matches the recorded SHA-256 value.':'Evidence digest does not match.',result!==true);}catch(err){showError(err);}}
});
$('actor-select').addEventListener('change',e=>{app.actor=e.target.value;toast(`Using configured identity: ${e.target.options[e.target.selectedIndex].text}`);});
$('settings-btn').addEventListener('click',()=>{$('token-input').value=app.token;$('settings-dialog').showModal();});
$('settings-form').addEventListener('submit',e=>{if(e.submitter?.value==='save'){app.token=$('token-input').value.trim();toast('Token updated for this tab');loadCatalog().then(render).catch(showError);}});
window.addEventListener('hashchange',()=>showView(location.hash.slice(1)||'overview'));
initialize();

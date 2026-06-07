var logs=[];
var selected=null;
var selectedLog=null;
var selectedEvents=[];
var mode=localStorage.getItem('llmTraceMode') || 'modern';
var copyBag=[];
var loadingDetail=false;
var listLimit=200;
var totalLogs=0;
var $=function(id){return document.getElementById(id)};

async function api(path){
  var r=await fetch('/_ui/api'+path,{cache:'no-store'});
  if(!r.ok){throw new Error((await r.text()) || r.statusText)}
  return r.json();
}
function fmtTime(t){var d=new Date(t); return isNaN(d.getTime()) ? String(t||'') : d.toLocaleString()}
function fmtShortTime(t){var d=new Date(t); return isNaN(d.getTime()) ? '' : d.toLocaleTimeString()}
function cls(code,err){if(err)return 'bad'; if(code>=500)return 'bad'; if(code>=400)return 'warn'; return 'ok'}
function escapeHtml(s){return String(s==null?'':s).replace(/[&<>"']/g,function(m){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#039;'}[m]})}
function short(s,n){s=String(s==null?'':s); n=n||160; return s.length>n ? s.slice(0,n)+'...' : s}
function parseJSON(s){
  if(s==null || s==='') return null;
  if(typeof s!=='string') return s;
  var t=s.trim();
  if(!t) return null;
  try{return JSON.parse(t)}catch(e){return null}
}
function pretty(value){
  var parsed=parseJSON(value);
  if(parsed!==null) return JSON.stringify(parsed,null,2);
  if(value==null) return '';
  if(typeof value==='object') return JSON.stringify(value,null,2);
  return String(value);
}
function parseJSONDetailed(value){
  if(value===undefined || value===null) return {ok:false,value:null};
  if(typeof value==='string'){
    var t=value.trim();
    var fenced=t.match(/^```(?:json|JSON)?\s*([\s\S]*?)\s*```$/);
    if(fenced) t=fenced[1].trim();
    if(!t) return {ok:false,value:null};
    try{return {ok:true,value:JSON.parse(t)}}catch(e){return {ok:false,value:null}}
  }
  if(typeof value==='object') return {ok:true,value:value};
  return {ok:false,value:null};
}
function isJSONContainer(v){return v!==null && typeof v==='object'}
var jsonViewerSeq=0;
function renderMaybeJSON(value,opts){
  opts=opts||{};
  var parsed=parseJSONDetailed(value);
  if(parsed.ok && isJSONContainer(parsed.value)){
    var id='jsonViewer'+(++jsonViewerSeq);
    var openDepth=opts.openDepth==null?1:opts.openDepth;
    var controls=opts.controls!==false;
    return '<div class="jsonViewer" id="'+id+'">'
      +(controls?'<div class="jsonToolbar"><span class="jsonBadge">JSON</span><button class="jsonToggle" onclick="event.stopPropagation();setJsonOpen(\''+id+'\',true)">Expand all</button><button class="jsonToggle" onclick="event.stopPropagation();setJsonOpen(\''+id+'\',false)">Collapse all</button></div>':'')
      +'<div class="jsonTree">'+renderJSONNode(parsed.value,undefined,false,0,openDepth)+'</div></div>';
  }
  var text=opts.pretty ? pretty(value) : (value==null?'':String(value));
  return opts.pre===false ? '<div class="text">'+escapeHtml(text)+'</div>' : '<pre>'+escapeHtml(text)+'</pre>';
}
function setJsonOpen(id,open){
  var root=$(id); if(!root) return;
  var nodes=root.querySelectorAll('details.jsonNode');
  for(var i=0;i<nodes.length;i++){nodes[i].open=open}
}
function jsonLabel(key,parentIsArray){
  if(key===undefined) return '';
  if(parentIsArray) return '<span class="jsonIndex">['+escapeHtml(key)+']</span><span class="jsonColon">: </span>';
  return '<span class="jsonKey">'+escapeHtml(JSON.stringify(String(key)))+'</span><span class="jsonColon">: </span>';
}
function renderJSONNode(value,key,parentIsArray,depth,openDepth){
  if(Array.isArray(value)){
    if(value.length===0) return '<div class="jsonLine">'+jsonLabel(key,parentIsArray)+'<span class="jsonBracket">[]</span></div>';
    return '<details class="jsonNode" '+(depth<openDepth?'open':'')+'><summary>'+jsonLabel(key,parentIsArray)+'<span class="jsonBracket">[</span><span class="jsonMeta">'+value.length+' items</span><span class="jsonBracket">]</span></summary><div class="jsonChildren">'+value.map(function(v,i){return renderJSONNode(v,i,true,depth+1,openDepth)}).join('')+'</div></details>';
  }
  if(isObj(value)){
    var keys=Object.keys(value);
    if(keys.length===0) return '<div class="jsonLine">'+jsonLabel(key,parentIsArray)+'<span class="jsonBracket">{}</span></div>';
    return '<details class="jsonNode" '+(depth<openDepth?'open':'')+'><summary>'+jsonLabel(key,parentIsArray)+'<span class="jsonBracket">{</span><span class="jsonMeta">'+keys.length+' keys</span><span class="jsonBracket">}</span></summary><div class="jsonChildren">'+keys.map(function(k){return renderJSONNode(value[k],k,false,depth+1,openDepth)}).join('')+'</div></details>';
  }
  return '<div class="jsonLine">'+jsonLabel(key,parentIsArray)+renderJSONPrimitive(value)+'</div>';
}
function renderJSONPrimitive(value){
  if(value===null) return '<span class="jsonNull">null</span>';
  if(value===undefined) return '<span class="jsonNull">undefined</span>';
  if(typeof value==='string') return '<span class="jsonString">'+escapeHtml(JSON.stringify(value))+'</span>';
  if(typeof value==='number') return '<span class="jsonNumber">'+escapeHtml(String(value))+'</span>';
  if(typeof value==='boolean') return '<span class="jsonBool">'+String(value)+'</span>';
  return '<span>'+escapeHtml(String(value))+'</span>';
}
function asArray(v){return Array.isArray(v)?v:[]}
function isObj(v){return v && typeof v==='object' && !Array.isArray(v)}
function field(obj,names){if(!isObj(obj))return undefined; for(var i=0;i<names.length;i++){if(obj[names[i]]!==undefined)return obj[names[i]]} return undefined}
function compact(arr){var out=[]; for(var i=0;i<arr.length;i++){if(arr[i]!==undefined && arr[i]!==null && arr[i]!=='' ) out.push(arr[i])} return out}
function copyId(text){copyBag.push(String(text==null?'':text)); return copyBag.length-1}
function copyStored(id){
  var text=copyBag[id]||'';
  if(navigator.clipboard && navigator.clipboard.writeText){navigator.clipboard.writeText(text)}
}
function copyButton(text,label){return '<button class="ghost" onclick="event.stopPropagation();copyStored('+copyId(text)+')">'+escapeHtml(label||'Copy')+'</button>'}

async function refreshLatest(){await loadAll(false)}
async function loadAll(selectNewest){
  try{
    var pack=await Promise.all([api('/health'),api('/stats'),api('/logs?limit='+listLimit)]);
    var health=pack[0], stats=pack[1], newLogs=pack[2]||[];
    var newest=newLogs[0] ? newLogs[0].request_uuid : null;
    logs=newLogs;
    $('health').textContent='Target '+health.target_url+' • DB '+health.db_path;
    totalLogs=stats.total_logs==null?0:stats.total_logs;
    $('sTotal').textContent=totalLogs;
    $('sToday').textContent=stats.today_logs==null?0:stats.today_logs;
    $('sErrors').textContent=stats.error_logs==null?0:stats.error_logs;
    $('sAvg').textContent=Math.round(stats.avg_duration_ms||0)+'ms';
    renderList();
    if(newest && (!selected || selectNewest)){
      show(newest);
    }else if(selected && selectedLog){
      reloadSelected();
    }
  }catch(e){
    $('list').innerHTML='<div class="empty">'+escapeHtml(e.message)+'</div>';
    $('detail').innerHTML='<div class="errorBox">Dashboard API unavailable. Start with USE_DB=true. '+escapeHtml(e.message)+'</div>';
    $('health').textContent='Start with USE_DB=true to enable trace APIs.';
  }
}
async function reloadSelected(){if(selected && !loadingDetail){await show(selected,true)}}
async function loadOlder(){
  try{
    var more=await api('/logs?limit='+listLimit+'&offset='+logs.length);
    logs=logs.concat(more||[]);
    renderList();
  }catch(e){
    $('list').insertAdjacentHTML('beforeend','<div class="errorBox">'+escapeHtml(e.message)+'</div>');
  }
}

function quickSummary(log){
  var body=parseJSON(log.body) || {};
  var tools=extractTools(body);
  var model=body.model || body.model_id || body.deployment || '';
  var messageCount=0;
  if(Array.isArray(body.messages)) messageCount=body.messages.length;
  else if(Array.isArray(body.input)) messageCount=body.input.length;
  else if(Array.isArray(body.contents)) messageCount=body.contents.length;
  else if(typeof body.input==='string') messageCount=1;
  var names=tools.map(function(t){return t.name}).filter(Boolean);
  if(names.length===0){names=toolNamesFromText((log.response||'')+' '+(log.body||''));}
  return {model:model, messageCount:messageCount, tools:tools, toolNames:names};
}
function toolNamesFromText(text){
  var names=[]; var seen={}; var re=/"(?:tool_name|name)"\s*:\s*"([^"]+)"/g; var m;
  while((m=re.exec(text||'')) && names.length<6){if(!seen[m[1]]){seen[m[1]]=true; names.push(m[1])}}
  return names;
}
function renderList(){
  var q=($('filter').value||'').toLowerCase();
  var items=[];
  for(var i=0;i<logs.length;i++){
    var l=logs[i];
    var s=quickSummary(l);
    var hay=(l.url+' '+l.body+' '+l.response+' '+s.model+' '+s.toolNames.join(' ')).toLowerCase();
    if(!q || hay.indexOf(q)>=0) items.push({log:l,summary:s});
  }
  var rows=items.map(function(item){
    var l=item.log, s=item.summary;
    var status=l.error?'ERR':(l.response_code||'-');
    var title=compact([s.model, s.messageCount? s.messageCount+' messages':'', s.tools.length? s.tools.length+' request tools':'']).join(' • ');
    var toolLine=s.toolNames.length?'<span class="toolnames">tools: '+escapeHtml(short(s.toolNames.join(', '),80))+'</span>':'';
    return '<div class="row '+(selected===l.request_uuid?'active':'')+'" onclick="show(\''+l.request_uuid+'\')">'
      +'<div class="rowhead"><span class="method">'+escapeHtml(l.method)+'</span><span class="chip dim">'+(l.is_stream?'stream':'http')+'</span><span class="status '+cls(l.response_code,l.error)+'">'+escapeHtml(status)+'</span></div>'
      +'<div class="url">'+escapeHtml(l.url)+'</div>'
      +(title?'<div class="rowtitle">'+escapeHtml(title)+'</div>':'')
      +'<div class="meta"><span>'+fmtShortTime(l.timestamp)+'</span><span>'+escapeHtml(String(l.duration_ms||0))+'ms</span>'+toolLine+'</div>'
      +'</div>';
  }).join('');
  if(rows && !q && totalLogs>logs.length){rows += '<div class="row"><button class="ghost" onclick="event.stopPropagation();loadOlder()">Load older requests ('+logs.length+'/'+totalLogs+')</button></div>';}
  $('list').innerHTML=rows || '<div class="empty">No matching calls</div>';
}

async function show(uuid,silent){
  selected=uuid; renderList(); loadingDetail=true;
  if(!silent) $('detail').innerHTML='<div class="loading">Loading trace...</div>';
  try{
    var pack=await Promise.all([api('/logs/'+uuid),api('/logs/'+uuid+'/events')]);
    selectedLog=pack[0]; selectedEvents=pack[1]||[];
    renderDetail();
  }catch(e){
    $('detail').innerHTML='<div class="errorBox">'+escapeHtml(e.message)+'</div>';
  }finally{loadingDetail=false;}
}
function setMode(next){mode=next; localStorage.setItem('llmTraceMode',mode); renderDetail()}
function setModernOpen(open){
  var root=$('detail'); if(!root) return;
  var nodes=root.querySelectorAll('details.foldSection,details.message.foldable,.toolBody details,.event details,details.event,details.jsonNode');
  for(var i=0;i<nodes.length;i++){nodes[i].open=open}
}
function collapseModernButton(){return '<button class="ghost" onclick="event.stopPropagation();setModernOpen(false)">Collapse all</button>'}
function renderDetail(){
  if(!selectedLog) return;
  copyBag=[];
  jsonViewerSeq=0;
  var trace=buildTrace(selectedLog,selectedEvents);
  var l=selectedLog;
  var status=l.error?'ERR':(l.response_code||'-');
  var model=trace.request.model || trace.response.model || '';
  var provider=trace.provider || 'unknown';
  var toolCount=trace.request.tools.length + trace.response.toolCalls.length;
  $('detail').innerHTML=''
    +'<div class="hero"><div><h2>'+escapeHtml(l.method+' '+l.url)+'</h2><div class="sub">'+fmtTime(l.timestamp)+' • '+escapeHtml(String(l.duration_ms||0))+'ms • request '+escapeHtml(l.request_uuid)+'</div></div>'
    +'<div class="pills"><span class="pill">status <b class="'+cls(l.response_code,l.error)+'">'+escapeHtml(status)+'</b></span><span class="pill">'+(l.is_stream?'SSE stream':'regular HTTP')+'</span><span class="pill">provider '+escapeHtml(provider)+'</span>'+(model?'<span class="pill">model '+escapeHtml(model)+'</span>':'')+'<span class="pill">tools '+toolCount+'</span></div></div>'
    +'<div class="tabs"><div class="tabgroup"><button class="tab '+(mode==='modern'?'active':'')+'" onclick="setMode(\'modern\')">Modern trace</button><button class="tab '+(mode==='raw'?'active':'')+'" onclick="setMode(\'raw\')">Raw payload</button></div><div class="actions">'+(mode==='modern'?collapseModernButton():'')+copyButton(trace.raw.requestText+'\n\n'+trace.raw.responseText,'Copy raw')+copyButton(trace.response.text || selectedLog.response || '', 'Copy answer')+'</div></div>'
    +(mode==='raw'?renderRaw(trace):renderModern(trace));
}

function buildTrace(log,events){
  var requestBody=parseJSON(log.body);
  var responseBody=parseJSON(log.response);
  var eventObjs=parseEventObjects(events);
  var provider=detectProvider(log.url,requestBody,responseBody,eventObjs);
  var req=extractRequest(requestBody);
  var resp=events && events.length ? extractStreamingResponse(eventObjs,responseBody) : extractRegularResponse(responseBody);
  if(!resp.text && responseBody && responseBody.processed_text && !resp.reasoning && resp.toolCalls.length===0) resp.text=responseBody.processed_text;
  if(resp.toolCalls.length===0 && responseBody && responseBody.tool_name){resp.toolCalls.push({name:responseBody.tool_name,args:'',id:''})}
  return {log:log,events:events||[],eventObjs:eventObjs,provider:provider,request:req,response:resp,raw:{headersText:pretty(log.headers),requestText:pretty(log.body),responseText:rawResponseText(log,events),storedResponseText:pretty(log.response)}};
}
function detectProvider(url,body,response,eventObjs){
  var u=String(url||'').toLowerCase();
  if(u.indexOf('/messages')>=0 || (body && body.anthropic_version) || (response && Array.isArray(response.content))) return 'anthropic';
  if(u.indexOf('/chat/completions')>=0 || (body && Array.isArray(body.messages))) return 'openai-chat';
  if(u.indexOf('/responses')>=0 || (body && body.input!==undefined) || (response && Array.isArray(response.output))) return 'openai-responses';
  if(u.indexOf('generatecontent')>=0 || (body && Array.isArray(body.contents))) return 'gemini';
  for(var i=0;i<eventObjs.length;i++){var t=eventObjs[i].type||eventObjs[i].event_type||''; if(String(t).indexOf('content_block')>=0) return 'anthropic'; if(String(t).indexOf('response.')===0) return 'openai-responses'}
  return 'generic';
}
function parseEventObjects(events){
  return asArray(events).map(function(e){
    var obj=parseJSON(e.data);
    return {raw:e,json:obj,event_type:e.event_type,type:(obj && obj.type) || e.event_type || 'message',data:e.data,sequence:e.sequence,timestamp:e.timestamp};
  });
}

function extractRequest(body){
  var req={model:'',settings:{},system:[],messages:[],tools:[],raw:body};
  if(!body){return req}
  req.model=body.model || body.model_id || body.deployment || '';
  var settingsKeys=['stream','temperature','top_p','top_k','max_tokens','max_completion_tokens','max_output_tokens','stop','tool_choice','parallel_tool_calls','response_format','reasoning','reasoning_effort','thinking','metadata'];
  settingsKeys.forEach(function(k){if(body[k]!==undefined) req.settings[k]=body[k]});
  if(body.system!==undefined) req.system.push({role:'system',parts:contentToParts(body.system)});
  if(body.instructions!==undefined) req.system.push({role:'instructions',parts:contentToParts(body.instructions)});
  if(Array.isArray(body.messages)){
    body.messages.forEach(function(m,i){req.messages.push(messageFromObject(m,i))});
  }else if(Array.isArray(body.input)){
    body.input.forEach(function(m,i){req.messages.push(messageFromObject(m,i))});
  }else if(typeof body.input==='string'){
    req.messages.push({role:'user',name:'input',parts:contentToParts(body.input),index:0,raw:body.input});
  }else if(Array.isArray(body.contents)){
    body.contents.forEach(function(m,i){req.messages.push(geminiMessageFromObject(m,i))});
  }
  req.tools=extractTools(body);
  return req;
}
function messageFromObject(m,i){
  if(!isObj(m)) return {role:'item',parts:contentToParts(m),index:i,raw:m};
  if(m.type==='function_call'){
    return {role:m.role||'assistant',name:m.name||m.call_id||'function_call',parts:[{type:'tool_call',name:m.name||'function_call',id:m.call_id||m.id||'',text:m.arguments||'',json:m}],index:i,raw:m};
  }
  if(m.type==='function_call_output'){
    return {role:m.role||'tool',name:m.call_id||'function_call_output',parts:[{type:'tool_result',name:m.call_id||'function_call_output',id:m.call_id||'',text:m.output==null?'':String(m.output),json:m}],index:i,raw:m};
  }
  if(m.type==='reasoning'){
    return {role:m.role||'reasoning',name:m.id||'',parts:[{type:'reasoning',id:m.id||'',text:reasoningTextFromObject(m,true),json:m}],index:i,raw:m};
  }
  var role=m.role || m.type || 'message';
  var content=m.content!==undefined ? m.content : (m.text!==undefined ? m.text : (m.output!==undefined ? m.output : m.input));
  var parts=contentToParts(content);
  if(m.tool_calls) asArray(m.tool_calls).forEach(function(tc){parts.push(toolCallPart(tc))});
  if(m.function_call) parts.push({type:'tool_call',name:m.function_call.name||'function_call',text:m.function_call.arguments||'',json:m.function_call});
  if(m.tool_call_id && role==='tool') parts.unshift({type:'tool_result_meta',name:m.tool_call_id,text:'tool_call_id: '+m.tool_call_id});
  return {role:role,name:m.name||m.tool_call_id||'',parts:parts,index:i,raw:m};
}
function reasoningTextFromObject(item,withPlaceholder){
  var parts=[];
  function addText(v){if(v!==undefined && v!==null && String(v)!=='') parts.push(String(v))}
  addText(item.text);
  asArray(item.summary).forEach(function(s){
    if(typeof s==='string') addText(s);
    else if(isObj(s)) addText(s.text!==undefined?s.text:(s.summary_text!==undefined?s.summary_text:''));
  });
  asArray(item.content).forEach(function(c){
    if(typeof c==='string') addText(c);
    else if(isObj(c)) addText(c.text!==undefined?c.text:(c.summary_text!==undefined?c.summary_text:''));
  });
  if(parts.length) return parts.join('\n\n');
  if(!withPlaceholder) return '';
  var hints=[];
  if(Array.isArray(item.summary) && item.summary.length===0) hints.push('summary is empty');
  if(Array.isArray(item.content) && item.content.length===0) hints.push('content is empty');
  if(item.encrypted_content) hints.push('encrypted reasoning content is present but cannot be displayed');
  return 'No reasoning summary text available'+(hints.length?' ('+hints.join('; ')+')':'')+'.';
}
function geminiMessageFromObject(m,i){
  var role=(m && m.role) || 'user';
  var parts=[];
  asArray(m && m.parts).forEach(function(p){
    if(p.text!==undefined) parts.push({type:'text',text:p.text});
    else if(p.functionCall) parts.push({type:'tool_call',name:p.functionCall.name,text:JSON.stringify(p.functionCall.args||{},null,2),json:p.functionCall});
    else if(p.functionResponse) parts.push({type:'tool_result',name:p.functionResponse.name,text:JSON.stringify(p.functionResponse.response||{},null,2),json:p.functionResponse});
    else parts.push({type:'json',text:JSON.stringify(p,null,2),json:p});
  });
  return {role:role,parts:parts,index:i,raw:m};
}
function contentToParts(content){
  if(content===undefined || content===null) return [];
  if(typeof content==='string') return [{type:'text',text:content}];
  if(Array.isArray(content)){
    var parts=[]; content.forEach(function(c){parts=parts.concat(contentToParts(c))}); return parts;
  }
  if(isObj(content)){
    var t=content.type || '';
    if(content.text!==undefined) return [{type:t==='thinking'?'reasoning':'text',text:String(content.text),json:content}];
    if(content.input_text!==undefined) return [{type:'text',text:String(content.input_text),json:content}];
    if(content.output_text!==undefined) return [{type:'text',text:String(content.output_text),json:content}];
    if(content.refusal!==undefined) return [{type:'refusal',text:String(content.refusal),json:content}];
    if(t==='tool_use' || content.name && content.input!==undefined) return [{type:'tool_call',name:content.name||'tool',id:content.id||'',text:JSON.stringify(content.input||{},null,2),json:content}];
    if(t==='tool_result' || content.tool_use_id) return [{type:'tool_result',name:content.tool_use_id||content.name||'tool_result',id:content.tool_use_id||'',text:typeof content.content==='string'?content.content:JSON.stringify(content.content||content.result||{},null,2),json:content}];
    if(content.image_url) return [{type:'image',text:typeof content.image_url==='string'?content.image_url:(content.image_url.url||JSON.stringify(content.image_url)),json:content}];
    if(t.indexOf('image')>=0) return [{type:'image',text:JSON.stringify(content,null,2),json:content}];
    if(content.functionCall) return [{type:'tool_call',name:content.functionCall.name,text:JSON.stringify(content.functionCall.args||{},null,2),json:content.functionCall}];
    if(content.functionResponse) return [{type:'tool_result',name:content.functionResponse.name,text:JSON.stringify(content.functionResponse.response||{},null,2),json:content.functionResponse}];
    return [{type:t||'json',text:JSON.stringify(content,null,2),json:content}];
  }
  return [{type:'text',text:String(content)}];
}
function toolCallPart(tc){
  if(!isObj(tc)) return {type:'tool_call',name:'tool',text:String(tc)};
  var fn=tc.function || tc;
  return {type:'tool_call',name:fn.name || tc.name || tc.type || 'tool',id:tc.id||tc.call_id||'',text:fn.arguments || JSON.stringify(fn.input||tc.input||{},null,2),json:tc};
}
function extractTools(body){
  var tools=[];
  function addTool(raw){
    if(!raw) return;
    if(raw.function_declarations){asArray(raw.function_declarations).forEach(addTool); return;}
    var fn=raw.function || raw;
    var name=fn.name || raw.name || raw.type || 'tool';
    tools.push({name:name,type:raw.type||fn.type||'function',description:fn.description||raw.description||'',schema:fn.parameters||raw.parameters||fn.input_schema||raw.input_schema||raw.schema||{},raw:raw});
  }
  asArray(body.tools).forEach(addTool);
  asArray(body.functions).forEach(addTool);
  if(Array.isArray(body.tool_config && body.tool_config.function_declarations)) asArray(body.tool_config.function_declarations).forEach(addTool);
  return tools;
}

function extractRegularResponse(obj){
  var resp={model:'',text:'',reasoning:'',toolCalls:[],usage:null,finish:'',raw:obj};
  if(!obj){return resp}
  resp.model=obj.model || '';
  resp.usage=obj.usage || obj.usageMetadata || null;
  resp.finish=obj.stop_reason || obj.finish_reason || '';
  if(typeof obj.output_text==='string') resp.text += obj.output_text;
  if(Array.isArray(obj.content)){
    obj.content.forEach(function(c){
      if(c.type==='thinking') resp.reasoning += (c.text||'');
      else if(c.type==='tool_use') resp.toolCalls.push({name:c.name,id:c.id,args:JSON.stringify(c.input||{},null,2),raw:c});
      else if(c.type==='text' || c.text!==undefined) resp.text += (c.text||'');
    });
  }
  if(Array.isArray(obj.choices)){
    obj.choices.forEach(function(ch){
      if(ch.finish_reason) resp.finish=ch.finish_reason;
      var m=ch.message || ch.delta || {};
      if(typeof m.content==='string') resp.text += m.content;
      else if(Array.isArray(m.content)) contentToParts(m.content).forEach(function(p){if(p.type==='text') resp.text += p.text; else if(p.type==='reasoning') resp.reasoning += p.text; else if(p.type==='tool_call') resp.toolCalls.push({name:p.name,id:p.id,args:p.text,raw:p.json})});
      if(m.reasoning_content) resp.reasoning += m.reasoning_content;
      asArray(m.tool_calls).forEach(function(tc){var fn=tc.function||{}; resp.toolCalls.push({name:fn.name||tc.name,id:tc.id||'',args:fn.arguments||'',raw:tc})});
    });
  }
  if(Array.isArray(obj.output)){
    obj.output.forEach(function(item){
      if(item.type==='message') asArray(item.content).forEach(function(c){if(c.text) resp.text += c.text; if(c.output_text) resp.text += c.output_text; if(c.refusal) resp.text += c.refusal});
      else if(item.type==='function_call') resp.toolCalls.push({name:item.name,id:item.call_id||item.id||'',args:item.arguments||'',raw:item});
      else if(item.type==='reasoning') {var reasoningText=reasoningTextFromObject(item,false); if(reasoningText) resp.reasoning += (resp.reasoning?'\n\n':'')+reasoningText;}
    });
  }
  if(Array.isArray(obj.candidates)){
    obj.candidates.forEach(function(c){
      if(c.finishReason) resp.finish=c.finishReason;
      asArray(c.content && c.content.parts).forEach(function(p){
        if(p.text) resp.text += p.text;
        if(p.functionCall) resp.toolCalls.push({name:p.functionCall.name,id:'',args:JSON.stringify(p.functionCall.args||{},null,2),raw:p.functionCall});
      });
    });
  }
  if(!resp.text && typeof obj.text==='string') resp.text=obj.text;
  if(!resp.text && typeof obj.message==='string') resp.text=obj.message;
  return resp;
}
function extractStreamingResponse(eventObjs,stored){
  var resp={model:'',text:'',reasoning:'',toolCalls:[],usage:null,finish:'',raw:eventObjs};
  var toolMap={};
  var toolIndexKey={};
  function hasIndex(index){return index!==undefined && index!==null && index!==''}
  function keyFor(id,index,name){
    if(hasIndex(index) && toolIndexKey[index]) return toolIndexKey[index];
    if(id) return 'id:'+id;
    if(hasIndex(index)) return 'idx:'+index;
    if(name) return 'name:'+name;
    return 'anon:'+Object.keys(toolMap).length;
  }
  function getTool(id,index,name){
    var k=keyFor(id,index,name);
    if(!toolMap[k]) toolMap[k]={name:name||'',id:id||'',index:index,args:'',raw:[]};
    if(hasIndex(index)) toolIndexKey[index]=k;
    if(name) toolMap[k].name=name; if(id) toolMap[k].id=id; if(index!==undefined) toolMap[k].index=index;
    return toolMap[k];
  }
  eventObjs.forEach(function(ev){
    var o=ev.json; var et=ev.event_type || ev.type || '';
    if(ev.data==='[DONE]') return;
    if(!o){ if(et==='message' && ev.data) resp.text += ev.data; return; }
    if(o.model) resp.model=o.model;
    if(o.message){
      if(o.message.model) resp.model=o.message.model;
      if(o.message.usage) resp.usage=o.message.usage;
      if(o.message.stop_reason) resp.finish=o.message.stop_reason;
    }
    if(o.usage || (o.response && o.response.usage)) resp.usage=o.usage || o.response.usage;
    if(o.stop_reason || o.finish_reason || (o.delta && o.delta.stop_reason)) resp.finish=o.stop_reason || o.finish_reason || o.delta.stop_reason;
    if(o.type) et=o.type;

    asArray(o.choices).forEach(function(ch){
      if(ch.finish_reason) resp.finish=ch.finish_reason;
      var d=ch.delta || ch.message || {};
      if(d.content) resp.text += d.content;
      if(d.reasoning_content) resp.reasoning += d.reasoning_content;
      asArray(d.tool_calls).forEach(function(tc){
        var fn=tc.function||{}; var t=getTool(tc.id,tc.index,fn.name||tc.name); t.args += fn.arguments || ''; t.raw.push(o);
      });
    });

    if(o.delta){
      if(typeof o.delta==='string'){
        if(et.indexOf('reasoning')>=0) resp.reasoning += o.delta;
        else if(et.indexOf('function_call_arguments')>=0 || et.indexOf('arguments')>=0){getTool(o.call_id||o.item_id,o.output_index||o.index,o.name).args += o.delta}
        else resp.text += o.delta;
      }else if(isObj(o.delta)){
        if(o.delta.text) resp.text += o.delta.text;
        if(o.delta.thinking) resp.reasoning += o.delta.thinking;
        if(o.delta.partial_json) getTool(o.content_block && o.content_block.id,o.index,o.name).args += o.delta.partial_json;
      }
    }
    if(et.indexOf('reasoning')>=0){
      var reasonDone=o.text || (o.part && o.part.text) || '';
      if(reasonDone && resp.reasoning.indexOf(reasonDone)<0) resp.reasoning += (resp.reasoning?'\n\n':'')+reasonDone;
    }
    if((et.indexOf('function_call_arguments')>=0 || et.indexOf('arguments')>=0) && o.arguments){
      var tArgs=getTool(o.call_id||o.item_id,o.output_index||o.index,o.name); tArgs.args=o.arguments; tArgs.raw.push(o);
    }
    if(o.content_block){
      var cb=o.content_block;
      if(cb.type==='text' && cb.text) resp.text += cb.text;
      if(cb.type==='tool_use') {var t1=getTool(cb.id,o.index,cb.name); if(cb.input) t1.args += JSON.stringify(cb.input); t1.raw.push(o)}
    }
    if(o.item){
      var item=o.item;
      if(item.type==='function_call'){var t2=getTool(item.call_id||item.id,o.output_index||o.index,item.name); if(item.arguments) t2.args=item.arguments; t2.raw.push(o)}
      if(item.type==='reasoning'){var itemReasoning=reasoningTextFromObject(item,false); if(itemReasoning && resp.reasoning.indexOf(itemReasoning)<0) resp.reasoning += (resp.reasoning?'\n\n':'')+itemReasoning}
      if(item.type==='message') asArray(item.content).forEach(function(c){if(c.text) resp.text += c.text; if(c.output_text) resp.text += c.output_text});
    }
    if(o.response){
      var rr=extractRegularResponse(o.response); if(!resp.text) resp.text += rr.text; if(!resp.reasoning) resp.reasoning += rr.reasoning; if(rr.usage) resp.usage=rr.usage; if(rr.finish) resp.finish=rr.finish; rr.toolCalls.forEach(function(t){var tt=getTool(t.id,null,t.name); if(t.args) tt.args=t.args});
    }
    var full=extractRegularResponse(o);
    if(!o.response && full.text && et.indexOf('delta')<0 && et.indexOf('content_block')<0 && et.indexOf('reasoning')<0 && et.indexOf('function_call_arguments')<0 && et.indexOf('arguments')<0) resp.text += full.text;
    full.toolCalls.forEach(function(t){var tt=getTool(t.id,null,t.name); if(t.args && !tt.args) tt.args=t.args});
  });
  Object.keys(toolMap).forEach(function(k){resp.toolCalls.push(toolMap[k])});
  if(stored && stored.processed_text && !resp.text && !resp.reasoning && resp.toolCalls.length===0) resp.text=stored.processed_text;
  if(stored && stored.tool_name && resp.toolCalls.length===0) resp.toolCalls.push({name:stored.tool_name,args:'',id:''});
  return resp;
}

function renderFoldSection(title,meta,body,opts){
  opts=opts||{};
  var classes='section foldSection'+(opts.full?' full':'');
  var bodyClasses='sectionBody'+(opts.noMax?' noMax':'');
  var open=opts.open===true?' open':'';
  var metaHtml=meta?'<span class="sectionMeta">'+escapeHtml(meta)+'</span>':'';
  return '<details class="'+classes+'"'+open+'><summary class="sectionHead"><span class="sectionHeadTitle"><span class="foldCaret" aria-hidden="true"></span><span class="sectionTitle">'+escapeHtml(title)+'</span></span>'+metaHtml+'</summary><div class="'+bodyClasses+'">'+body+'</div></details>';
}
function renderModern(trace){
  var req=trace.request, resp=trace.response;
  return '<div class="tracegrid">'
    +renderFoldSection('Request trace',req.messages.length+' messages • '+req.tools.length+' tools','<div class="stack">'+renderSettings(req,trace)+renderSystem(req)+renderMessages(req.messages)+'</div>',{noMax:true})
    +renderFoldSection('Response trace',trace.log.is_stream?trace.events.length+' stream events':'HTTP response','<div class="stack">'+renderResponse(resp,trace)+'</div>',{noMax:true})
    +renderFoldSection('Tools available to the model','request tool schemas',renderTools(req.tools),{full:true,noMax:true})
    +renderFoldSection('Stream timeline','raw events grouped by sequence',renderCompactEvents(trace.events),{full:true})
    +'</div>';
}
function renderSettings(req,trace){
  var rows=[]; if(req.model) rows.push(['model',req.model]); rows.push(['provider',trace.provider]); rows.push(['url',trace.log.url]);
  Object.keys(req.settings).forEach(function(k){rows.push([k,typeof req.settings[k]==='string'?req.settings[k]:JSON.stringify(req.settings[k])])});
  return renderFoldSection('Model request','','<div class="kv">'+rows.map(function(r){return '<div>'+escapeHtml(r[0])+'</div><div>'+escapeHtml(r[1])+'</div>'}).join('')+'</div>');
}
function renderSystem(req){
  if(!req.system.length) return '';
  return req.system.map(function(m){return renderMessageBlock(m,'system','')}).join('');
}
function renderMessages(messages){
  if(!messages.length) return '<div class="empty">No prompt messages found in request body.</div>';
  return messages.map(function(m){
    var roleClass=['system','developer','user','assistant','tool','reasoning'].indexOf(m.role)>=0?m.role:'other';
    var meta='<span class="muted">#'+escapeHtml(m.index)+'</span>'+(m.name?'<span class="chip dim">'+escapeHtml(m.name)+'</span>':'');
    return renderMessageBlock(m,roleClass,meta);
  }).join('');
}
function renderMessageBlock(m,roleClass,metaHtml){
  var partCount=(m.parts&&m.parts.length)||0;
  var countLabel=partCount ? '<span class="muted foldHint">'+partCount+' part'+(partCount===1?'':'s')+'</span>' : '<span class="muted foldHint">empty</span>';
  return '<details class="message foldable"><summary class="messageTop"><span class="role '+roleClass+'">'+escapeHtml(m.role)+'</span>'+(metaHtml||'')+countLabel+'</summary><div class="messageBody">'+renderParts(m.parts)+'</div></details>';
}
function renderParts(parts){
  if(!parts || !parts.length) return '<span class="muted">empty</span>';
  return parts.map(function(p){
    var cls='part'; if(p.type==='text') cls+=' textpart'; else if(p.type==='tool_call') cls+=' toolcall'; else if(p.type==='tool_result' || p.type==='tool_result_meta') cls+=' toolresult'; else if(p.type==='reasoning') cls+=' reasoning'; else if(p.type==='image') cls+=' image';
    var head=p.type || 'content'; if(p.name) head+=' • '+p.name; if(p.id) head+=' • '+p.id;
    return '<div class="'+cls+'"><div class="partHead">'+escapeHtml(head)+'</div>'+renderMaybeJSON(p.text||'',{pre:false,controls:true,openDepth:1})+'</div>';
  }).join('');
}
function renderTools(tools){
  if(!tools.length) return '<div class="empty">No tool schemas in this request.</div>';
  return '<div class="stack">'+tools.map(function(t){return '<div class="toolCard"><div class="toolCardTop"><div><div class="toolName">'+escapeHtml(t.name)+'</div>'+(t.description?'<div class="toolDesc">'+escapeHtml(t.description)+'</div>':'')+'</div><span class="chip dim">'+escapeHtml(t.type||'tool')+'</span></div><div class="toolBody"><details><summary class="muted">schema</summary>'+renderMaybeJSON(t.schema||t.raw,{openDepth:2})+'</details></div></div>'}).join('')+'</div>';
}
function renderResponse(resp,trace){
  var html='';
  if(trace.log.error) html+='<div class="errorBox">'+escapeHtml(trace.log.error)+'</div>';
  if(resp.text) html+='<div class="answer"><h3>Assistant output</h3>'+renderMaybeJSON(resp.text,{pre:false,controls:true,openDepth:1})+'</div>';
  else html+='<div class="empty">No assistant text extracted. Use Raw payload for the original response.</div>';
  if(resp.reasoning) html+='<div class="part reasoning"><div class="partHead">Reasoning</div>'+renderMaybeJSON(resp.reasoning,{pre:false,controls:true,openDepth:1})+'</div>';
  if(resp.toolCalls.length){
    html+='<div class="stack">'+resp.toolCalls.map(function(t){var args=t.args||''; var parsed=parseJSON(args); var argText=parsed!==null?JSON.stringify(parsed,null,2):args; return '<div class="toolCallOut"><div class="toolCallOutTop"><div><b>'+escapeHtml(t.name||'tool')+'</b>'+(t.id?'<div class="tiny">'+escapeHtml(t.id)+'</div>':'')+'</div><span class="chip">tool call</span></div><div class="toolCallOutBody">'+renderMaybeJSON(argText,{openDepth:2})+'</div></div>'}).join('')+'</div>';
  }
  var rows=[]; if(resp.model) rows.push(['model',resp.model]); if(resp.finish) rows.push(['finish',resp.finish]); if(resp.usage) rows.push(['usage',JSON.stringify(resp.usage)]); rows.push(['stream events',String(trace.events.length)]);
  html+='<div class="section"><div class="sectionHead"><div class="sectionTitle">Response metadata</div></div><div class="sectionBody"><div class="kv">'+rows.map(function(r){return '<div>'+escapeHtml(r[0])+'</div><div>'+escapeHtml(r[1])+'</div>'}).join('')+'</div></div></div>';
  return html;
}
function renderCompactEvents(events){
  if(!events.length) return '<div class="empty">No SSE events captured for this request.</div>';
  var counts={}; events.forEach(function(e){counts[e.event_type]=(counts[e.event_type]||0)+1});
  var chips=Object.keys(counts).map(function(k){return '<span class="chip">'+escapeHtml(k)+' '+counts[k]+'</span>'}).join('');
  return '<div class="eventStats">'+chips+'</div><div style="height:12px"></div><div class="stack">'+events.map(function(e){return '<div class="event"><details><summary>#'+escapeHtml(e.sequence)+' '+escapeHtml(e.event_type)+' <span class="tiny">'+fmtTime(e.timestamp)+'</span> '+copyButton(eventToSSE(e),'Copy event')+'</summary><div class="eventBody">'+renderMaybeJSON(e.data,{pretty:true,openDepth:1})+'</div></details></div>'}).join('')+'</div>';
}
function renderRaw(trace){
  var events=trace.events;
  var eventDetails=events.map(function(e){return '<details class="event"><summary>#'+escapeHtml(e.sequence)+' '+escapeHtml(e.event_type)+' <span class="tiny">'+fmtTime(e.timestamp)+'</span> '+copyButton(eventToSSE(e),'Copy event')+'</summary><div class="eventBody">'+renderMaybeJSON(e.data,{pretty:true,openDepth:1})+'</div></details>'}).join('');
  return '<div class="rawgrid">'
    +'<details class="rawBlock"><summary>Original request headers '+copyButton(trace.raw.headersText,'Copy')+'</summary><div class="rawInner">'+renderMaybeJSON(trace.raw.headersText,{pretty:true,openDepth:2})+'</div></details>'
    +'<details class="rawBlock"><summary>Original request body '+copyButton(trace.raw.requestText,'Copy')+'</summary><div class="rawInner">'+renderMaybeJSON(trace.raw.requestText,{pretty:true,openDepth:2})+'</div></details>'
    +'<details class="rawBlock"><summary>Original response '+(trace.log.is_stream?'reconstructed SSE':'body')+' '+copyButton(trace.raw.responseText,'Copy')+'</summary><div class="rawInner">'+renderMaybeJSON(trace.raw.responseText,{pretty:true,openDepth:2})+'</div></details>'
    +(trace.log.is_stream?'<details class="rawBlock"><summary>Stored response field '+copyButton(trace.raw.storedResponseText,'Copy')+'</summary><div class="rawInner">'+renderMaybeJSON(trace.raw.storedResponseText,{pretty:true,openDepth:2})+'</div></details>':'')
    +'<details class="rawBlock"><summary>SSE events '+events.length+'</summary><div class="rawInner"><div class="stack">'+(eventDetails||'<div class="empty">No stream events captured.</div>')+'</div></div></details>'
    +'</div>';
}
function rawResponseText(log,events){
  if(log.is_stream && events && events.length){return events.map(function(e){return eventToSSE(e)}).join('')}
  return pretty(log.response || log.error || '');
}
function eventToSSE(e){
  var lines=[];
  if(e.event_type && e.event_type!=='message') lines.push('event: '+e.event_type);
  String(e.data==null?'':e.data).split('\n').forEach(function(line){lines.push('data: '+line)});
  return lines.join('\n')+'\n\n';
}

loadAll(true);

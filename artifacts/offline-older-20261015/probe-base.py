"""Bounded offline experiment. Reads originals as bytes; opens only private DB copies."""
from pathlib import Path
import collections, hashlib, json, os, re, sqlite3, tempfile
from google.protobuf import descriptor_pb2, descriptor_pool, message_factory, json_format
ROOT=Path(__file__).resolve().parents[2]
SCHEMA=ROOT/'artifacts/proto-snapshots/agy-1.3.6-e14253aa79d4/trajectory.protoset'
STORE=Path.home()/'.gemini/antigravity-cli/conversations'
files={f.name:f for f in descriptor_pb2.FileDescriptorSet.FromString(SCHEMA.read_bytes()).file};pool=descriptor_pool.DescriptorPool();loaded=set()
def load(n):
 if n in loaded:return
 for dep in files[n].dependency:load(dep)
 pool.Add(files[n]);loaded.add(n)
for n in files:load(n)
def msg(n,b=b''):
 m=message_factory.GetMessageClass(pool.FindMessageTypeByName(n))();m.ParseFromString(b or b'');return m
def digest(b):return hashlib.sha256(b).hexdigest()
def unknown(m):
 c=type(m)();c.CopyFrom(m);c.DiscardUnknownFields();return c.SerializeToString(deterministic=True)!=m.SerializeToString(deterministic=True)
def jsonof(m):return json_format.MessageToDict(m,descriptor_pool=pool)
def capture(cid,folder):
 paths=[STORE/(cid+x) for x in ['.db','.db-wal','.trajectory.json']]
 before={p.name:p.read_bytes() if p.exists() else None for p in paths}
 for p in paths:
  if before[p.name] is not None:(folder/p.name).write_bytes(before[p.name])
 assert all((p.read_bytes() if p.exists() else None)==before[p.name] for p in paths),'source changed during copy'
 return paths,{n:digest(b) if b is not None else None for n,b in before.items()}
def decode(path):
 db=sqlite3.connect(path.as_uri()+'?mode=ro',uri=True);db.execute('PRAGMA query_only=ON');db.execute('BEGIN')
 assert db.execute('PRAGMA user_version').fetchone()[0]==1
 assert db.execute('PRAGMA quick_check').fetchone()[0]=='ok'
 t=msg('gemini_coder.Trajectory');rows=db.execute('SELECT trajectory_id,cascade_id,trajectory_type,source FROM trajectory_meta').fetchall();assert len(rows)==1
 t.trajectory_id,t.cascade_id,t.trajectory_type,t.source=rows[0];unknown_records=collections.Counter();redundant=0
 for row in db.execute('SELECT idx,step_type,status,has_subtrajectory,metadata,error_details,permissions,task_details,render_info,step_payload,step_format FROM steps ORDER BY idx'):
  idx,typ,status,sub,metadata,error,permissions,task,render,payload,fmt=row
  assert idx==len(t.steps) and not sub and fmt==0
  s=msg('gemini_coder.Step',payload);assert s.type==typ and s.status==status
  for name,b in [('metadata',metadata),('error',error),('permissions',permissions),('task_details',task)]:
   v=msg(s.DESCRIPTOR.fields_by_name[name].message_type.full_name,b);assert getattr(s,name)==v;redundant+=1
  nested=s.generic.result
  assert (render is not None)==nested.HasField('step_render_info')
  if render is not None:assert nested.step_render_info==msg('exa.cortex_pb.StepRenderInfo',render);redundant+=1
  if unknown(s):unknown_records['steps']+=1
  t.steps.add().CopyFrom(s)
 for table,name in [('gen_metadata','generator_metadata'),('executor_metadata','executor_metadatas'),('parent_references','parent_references'),('battle_mode_infos','battle_mode_infos')]:
  query='SELECT idx,data'+(',size' if table=='gen_metadata' else '')+' FROM '+table+' ORDER BY idx'
  for row in db.execute(query):
   idx,data=row[:2];assert idx==len(getattr(t,name))
   if table=='gen_metadata':assert row[2]==len(data)
   v=msg(t.DESCRIPTOR.fields_by_name[name].message_type.full_name,data)
   if unknown(v):unknown_records[table]+=1
   getattr(t,name).add().CopyFrom(v)
 rows=db.execute('SELECT id,data FROM trajectory_metadata_blob').fetchall();assert len(rows)==1 and rows[0][0]=='main'
 t.metadata.CopyFrom(msg(t.DESCRIPTOR.fields_by_name['metadata'].message_type.full_name,rows[0][1]));db.close()
 if unknown(t.metadata):unknown_records['trajectory_metadata_blob']+=1
 return t,dict(unknown_records),redundant
def scan_any(m, counts):
 if m.DESCRIPTOR.full_name=='google.protobuf.Any':
  counts['payloads']+=1
  inner=msg(m.type_url.rsplit('/',1)[-1],m.value)
  if unknown(inner):counts['with_unknown_fields']+=1
  scan_any(inner,counts)
  return
 for f,v in m.ListFields():
  if f.message_type is None:continue
  if f.is_repeated:
   if f.message_type.GetOptions().map_entry:
    if f.message_type.fields_by_name['value'].message_type:
     for child in v.values():scan_any(child,counts)
   else:
    for child in v:scan_any(child,counts)
  else:scan_any(v,counts)
results=[]
IDS=os.environ.get('AGY_AUDIT_SESSION_IDS', '').split()
if not IDS or any(not re.fullmatch(r'[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}', cid) for cid in IDS):
 raise SystemExit('Set AGY_AUDIT_SESSION_IDS to actual session UUIDs from your private store.')
for cid in IDS:
 with tempfile.TemporaryDirectory(prefix='agy-offline-') as tmp:
  folder=Path(tmp);paths,hashes=capture(cid,folder);t,u,checks=decode(folder/(cid+'.db'));side=json.loads((folder/(cid+'.trajectory.json')).read_bytes());side.pop('agyReader',None)
  any_counts=collections.Counter();scan_any(t,any_counts)
  raw=jsonof(t);r={'id':cid,'input_sha256':hashes,'steps_decoded':len(t.steps),'steps_sidecar':len(side.get('steps',[])),'unknown_wire_records':u,'embedded_any_scan':dict(any_counts),'redundant_columns_equal':checks}
  # Typed parsing normalizes JSON defaults, enum names, int64 spellings, and Any representation.
  st=msg('gemini_coder.Trajectory');json_format.ParseDict(side,st,descriptor_pool=pool);sj=jsonof(st)
  r['whole_message_equal']=t==st;r['top_level_fields_equal']=[k for k in sorted(raw.keys()|sj.keys()) if k!='steps' and raw.get(k)==sj.get(k)];r['top_level_fields_different']=[k for k in sorted(raw.keys()|sj.keys()) if k!='steps' and raw.get(k)!=sj.get(k)]
  projected_matches=0;embedded_matches=0;embedded_total=0;embedded_differences=collections.Counter();feature_counts=collections.Counter()
  def features(v):
   if isinstance(v,dict):
    for k,x in v.items():
     if k in {'retryInfos','retries','responseModel','responseModelFull','thinkingSignature','thinkingRedacted','usedFallbackSummary','modelSummarizationFailed','mergedStepIdxs','stepsCoveredByCheckpoint','checkpointIndex','cacheReadInputTokens','cacheCreationInputTokens'} and x:feature_counts[k]+=1
     features(x)
   elif isinstance(v,list):
    for x in v:features(x)
  features(raw)
  equal=0;changes=collections.Counter();transitions=collections.Counter();native_matches=collections.Counter()
  for a,b in zip(raw.get('steps',[]),sj.get('steps',[])):
   if a==b:equal+=1;native_matches[a.get('type','default')]+=1
   else:
    payload=a.get('generic',{}).get('result',{}).get('payload',{})
    if payload.get('@type','').endswith('/gemini_coder.Step'):
     embedded_total+=1
     projected={k:v for k,v in payload.items() if k!='@type'}
     for k in ('taskDetails','status'):
      if k in a:projected[k]=a[k]
     projected_matches+=(projected==b)
     embedded_matches+=({k:v for k,v in payload.items() if k!='@type'}==b)
     for k in (payload.keys()|b.keys())-{'@type'}:
      if payload.get(k)!=b.get(k):
       if k=='metadata':
        for mk in payload.get(k,{}).keys()|b.get(k,{}).keys():
         if payload.get(k,{}).get(mk)!=b.get(k,{}).get(mk):embedded_differences['metadata.'+mk]+=1
       else:embedded_differences[k]+=1
    transitions[a.get('type','default')+' -> '+b.get('type','default')]+=1
    for k in a.keys()|b.keys():
     if a.get(k)!=b.get(k):changes[k]+=1
  r.update(steps_equal_after_bounded_projection=equal+projected_matches,embedded_native_steps=embedded_total,embedded_native_exact_matches=embedded_matches,embedded_native_difference_fields=dict(embedded_differences),observed_field_presence_counts=dict(feature_counts),exact_step_matches=equal,step_field_differences=dict(changes),type_transitions=dict(transitions),exact_match_types=dict(native_matches))
  r['original_hashes_unchanged']=all((digest(p.read_bytes()) if p.exists() else None)==hashes[p.name] for p in paths);assert r['original_hashes_unchanged']
  results.append(r)
report={'schema_sha256':digest(SCHEMA.read_bytes()),'selection':'operator-supplied CLI session IDs; not random','daemon_calls':0,'originals_opened_with_sqlite':0,'results':results}
print(json.dumps(report,indent=2))

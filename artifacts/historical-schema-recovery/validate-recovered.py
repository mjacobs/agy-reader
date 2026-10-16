from pathlib import Path
import json,collections,tempfile,hashlib
from google.protobuf import descriptor_pb2 as d,descriptor_pool,message_factory,unknown_fields,json_format
r=Path(__file__).resolve().parent;repo=r.parents[1]
base=repo/'artifacts/offline-decode-20261015/probe.py';ns={'__file__':str(base)};exec(compile(base.read_text().split('results=[]')[0],str(base),'exec'),ns)
files={f.name:f for f in d.FileDescriptorSet.FromString((r/'1.0.10-trajectory.protoset').read_bytes()).file};pool=descriptor_pool.DescriptorPool();loaded=set()
def load(n):
 if n in loaded:return
 for dep in files[n].dependency:load(dep)
 pool.Add(files[n]);loaded.add(n)
for n in files:load(n)
targets={('exa.cortex_pb.CascadePlannerConfig',50):'exa.cortex_pb.ContextEngineConfig',('exa.chat_pb.ChatMessagePrompt',23):'exa.codeium_common_pb.Struct'}
ids=[x['id'] for x in json.loads((repo/'artifacts/offline-older-20261015/results.json').read_text())['results'] if x['unknown_field_identities']]
results=[]
for cid in ids:
 with tempfile.TemporaryDirectory(prefix='agy-history-') as temp:
  p=Path(temp);paths,hashes=ns['capture'](cid,p);t,_,_=ns['decode'](p/(cid+'.db'));counts=collections.Counter();configs=collections.Counter();struct_sizes=[]
  def visit(m):
   for u in unknown_fields.UnknownFieldSet(m):
    key=(m.DESCRIPTOR.full_name,u.field_number)
    if key not in targets:continue
    assert u.wire_type==2
    cls=message_factory.GetMessageClass(pool.FindMessageTypeByName(targets[key]));v=cls.FromString(u.data);clean=cls();clean.CopyFrom(v);clean.DiscardUnknownFields();assert clean==v,'recovered payload contains unknown fields'
    counts[targets[key]]+=1
    if key[1]==50:configs[json.dumps(json_format.MessageToDict(v),sort_keys=True)]+=1
    else:struct_sizes.append(len(v.fields))
   for f,v in m.ListFields():
    if not f.message_type:continue
    if f.is_repeated:
     if f.message_type.GetOptions().map_entry:
      if f.message_type.fields_by_name['value'].message_type:
       for c in v.values():visit(c)
     else:
      for c in v:visit(c)
    else:visit(v)
  visit(t)
  stable=all((ns['digest'](p.read_bytes()) if p.exists() else None)==hashes[p.name] for p in paths);assert stable
  results.append({'id':cid,'input_sha256':hashes,'recovered_payload_counts':dict(counts),'context_engine_config_shapes':dict(configs),'prompt_struct_field_counts':struct_sizes,'unknown_fields_in_recovered_messages':0,'original_hashes_unchanged':stable})
(r/'recovered-payload-validation.json').write_text(json.dumps({'daemon_calls':0,'historical_binary_executed':False,'results':results},indent=2)+'\n')
for x in results:print(x['id'][:8],x['recovered_payload_counts'],x['context_engine_config_shapes'],x['prompt_struct_field_counts'])

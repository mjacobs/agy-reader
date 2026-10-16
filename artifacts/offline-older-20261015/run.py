from pathlib import Path
import contextlib,io,json,re,hashlib,traceback,collections
from google.protobuf import unknown_fields
root=Path(__file__).resolve().parent
source=(root/'probe-base.py').read_text();selection=json.loads((root/'selection.json').read_text());results=[]
store=Path.home()/'.gemini/antigravity-cli/conversations'
def hashes(cid):
 return {ext:hashlib.sha256((store/(cid+ext)).read_bytes()).hexdigest() if (store/(cid+ext)).exists() else None for ext in ('.db','.db-wal','.trajectory.json')}
for sample in selection['samples']:
 cid=sample['id'];before=hashes(cid);ns={'__file__':str(root/'probe-base.py')};code=re.sub(r'^IDS=.*$', 'IDS='+repr([cid]),source,flags=re.M);out=io.StringIO()
 try:
  with contextlib.redirect_stdout(out):exec(compile(code,str(root/'probe-base.py'),'exec'),ns)
  result=json.loads(out.getvalue())['results'][0];result['decode_completed']=True
  unknown_counts=collections.Counter()
  def visit(m):
   for u in unknown_fields.UnknownFieldSet(m):unknown_counts[m.DESCRIPTOR.full_name+'.field_'+str(u.field_number)+'.wire_'+str(u.wire_type)]+=1
   if m.DESCRIPTOR.full_name=='google.protobuf.Any':
    visit(ns['msg'](m.type_url.rsplit('/',1)[-1],m.value));return
   for f,v in m.ListFields():
    if f.message_type is None:continue
    if f.is_repeated:
     if f.message_type.GetOptions().map_entry:
      if f.message_type.fields_by_name['value'].message_type:
       for child in v.values():visit(child)
     else:
      for child in v:visit(child)
    else:visit(v)
  visit(ns['t']);result['unknown_field_identities']=dict(unknown_counts)
  reparsed=ns['msg']('gemini_coder.Trajectory',ns['t'].SerializeToString(deterministic=True))
  result['protobuf_roundtrip_preserves_message_including_unknowns']=reparsed==ns['t']
  assert result['protobuf_roundtrip_preserves_message_including_unknowns']
 except Exception as e:
  result={'id':cid,'decode_completed':False,'error_type':type(e).__name__,'failure_locations':[{'function':f.name,'line':f.lineno} for f in traceback.extract_tb(e.__traceback__) if f.filename.endswith('probe-base.py')], 'input_sha256':before}
 result['original_hashes_unchanged']=hashes(cid)==before;assert result['original_hashes_unchanged'];result.update({k:sample[k] for k in ('created_at','db_mtime_utc')});results.append(result)
report={'schema_sha256':hashlib.sha256((root.parent/'proto-snapshots/agy-1.3.6-e14253aa79d4/trajectory.protoset').read_bytes()).hexdigest(),'daemon_calls':0,'originals_opened_with_sqlite':0,'results':results}
(root/'results.json').write_text(json.dumps(report,indent=2)+'\n')
for r in results:print(r['created_at'][:10],r['id'][:8],r.get('steps_decoded'),r.get('steps_equal_after_bounded_projection'),r.get('unknown_wire_records'),r.get('top_level_fields_different'),r.get('error_type'))


3google/cloud/aiplatform/v1beta1/agent_service.protogoogle.cloud.aiplatform.v1beta1google/api/annotations.protogoogle/api/client.protogoogle/api/field_behavior.protogoogle/api/policy.protogoogle/api/resource.protogoogle/api/versioning.protogoogle/api/visibility.proto8google/cloud/aiplatform/common/proto_field_options.proto+google/cloud/aiplatform/v1beta1/agent.proto/google/cloud/aiplatform/v1beta1/operation.proto4google/cloud/automl/common/proto_field_options.proto#google/longrunning/operations.proto google/protobuf/field_mask.proto.net/proto2/contrib/validator/annotations.proto<storage/datapol/annotations/proto/semantic_annotations.proto"Œ
CreateAgentRequestİ
parent (	BÄâAúA!aiplatform.googleapis.com/Agent  ğ˜Ì‚ÉÏ™`’óèç€(not empty($) ?: [REQUIRED_FIELD_MISSING]TisValidResourceName($, `Location`)?: [FIELD_INVALID] Invalid Location resource name.€‹±ÍRparent}
agent (2&.google.cloud.aiplatform.v1beta1.AgentB?âA’óèç"
 true ?: [REQUIRED_FIELD_MISSING]ÂöŒÜ"AUDIT¸Í¤‰
Ragent:ÀöÄğ	JR	idp_nonce"¤
CreateAgentOperationMetadatad
generic_metadata (29.google.cloud.aiplatform.v1beta1.GenericOperationMetadataRgenericMetadata:òœâ¡internal,ui,v1beta1,v1"ï
GetAgentRequestÓ
name (	B¾âAúA!
aiplatform.googleapis.com/Agent  ğ˜Ì‚ÉÏ™`’óèç{(not empty($) ?: [REQUIRED_FIELD_MISSING]OisValidResourceName($, `Agent`) ?: [FIELD_INVALID] Invalid Agent resource name.€‹±ÍRname:ÀöÄğ	"“
ListAgentsRequestŞ
parent (	BÅâAúA!aiplatform.googleapis.com/Agent  ğ˜Ì‚ÉÏ™`’óèç(not empty($) ?: [REQUIRED_FIELD_MISSING]UisValidResourceName($, `Location`) ?: [FIELD_INVALID] Invalid Location resource name.€‹±ÍRparentv
	page_size (BYâA  ğ˜ç’óèç;9$>=0 ?: [FIELD_INVALID] Page size should be non-negative.ÂöŒÜ"AUDITRpageSize*

page_token (	BâA  ğ˜çR	pageToken0
filter (	BâA  ğ˜íÂöŒÜ"AUDITRfilter3
order_by (	BâA  ğ˜íÂöŒÜ"AUDITRorderBy:ÀöÄğ	JJ"™
ListAgentsResponseR
agents (2&.google.cloud.aiplatform.v1beta1.AgentBÂöŒÜ"
AUDIT_SIZERagents/
next_page_token (	B  ğ˜çRnextPageToken"‰
SearchAgentsRequestŞ
parent (	BÅâAúA!aiplatform.googleapis.com/Agent  ğ˜Ì‚ÉÏ™`’óèç(not empty($) ?: [REQUIRED_FIELD_MISSING]UisValidResourceName($, `Location`) ?: [FIELD_INVALID] Invalid Location resource name.€‹±ÍRparentv
	page_size (BYâA  ğ˜ç’óèç;9$>=0 ?: [FIELD_INVALID] Page size should be non-negative.ÂöŒÜ"AUDITRpageSize*

page_token (	BâA  ğ˜çR	pageToken0
filter (	BâA  ğ˜íÂöŒÜ"AUDITRfilter3
order_by (	BâA  ğ˜íÂöŒÜ"AUDITRorderBy:ÀöÄğ	"›
SearchAgentsResponseR
agents (2&.google.cloud.aiplatform.v1beta1.AgentBÂöŒÜ"
AUDIT_SIZERagents/
next_page_token (	B  ğ˜çRnextPageToken"›
UpdateAgentRequest¥
agent (2&.google.cloud.aiplatform.v1beta1.AgentBæâA’óèçÈNnot empty($.name) ?: [REQUIRED_FIELD_MISSING] Agent resource name is required.TisValidResourceName($.name, `Agent`) ?: [FIELD_INVALID] Invalid Agent resource name.
 true ?: [REQUIRED_FIELD_MISSING]ÂöŒÜ"AUDIT€‹±ÍRagentU
update_mask (2.google.protobuf.FieldMaskBâA  ğ˜íÂöŒÜ"AUDITR
updateMask:ÀöÄğ	"¤
UpdateAgentOperationMetadatad
generic_metadata (29.google.cloud.aiplatform.v1beta1.GenericOperationMetadataRgenericMetadata:òœâ¡internal,ui,v1beta1,v1"
DeleteAgentRequestÓ
name (	B¾âAúA!
aiplatform.googleapis.com/Agent  ğ˜Ì‚ÉÏ™`’óèç{(not empty($) ?: [REQUIRED_FIELD_MISSING]OisValidResourceName($, `Agent`) ?: [FIELD_INVALID] Invalid Agent resource name.€‹±ÍRname
force (BâARforce:ÀöÄğ	"£
ListAgentMemoriesRequest×
parent (	B¾âAúA!
aiplatform.googleapis.com/Agent  ğ˜Ì‚ÉÏ™`’óèç{(not empty($) ?: [REQUIRED_FIELD_MISSING]OisValidResourceName($, `Agent`) ?: [FIELD_INVALID] Invalid Agent resource name.€‹±ÍRparentv
	page_size (BYâA  ğ˜ç’óèç;9$>=0 ?: [FIELD_INVALID] Page size should be non-negative.ÂöŒÜ"AUDITRpageSize*

page_token (	BâA  ğ˜çR	pageToken;
show_deleted (BâA  ğ˜çÂöŒÜ"AUDITRshowDeletedÕ
filter (	B¼âA  ğ˜í’óèçšempty($) or matches($, `(?i)^\s*category\s*(=|!=)\s*"[^"]*"\s*$`) ?: [FIELD_INVALID] Only a single predicate on `category` using `=` or `!=` is supported.ÂöŒÜ"AUDITRfilterÓ
view (20.google.cloud.aiplatform.v1beta1.AgentMemoryViewBŒâA  ğ˜ç’óèçnl$ != AGENT_MEMORY_VIEW_FULL ?: [FIELD_INVALID] AGENT_MEMORY_VIEW_FULL is not supported in ListAgentMemories.ÂöŒÜ"AUDITRview:òœâ¡internal,v1beta1ÀöÄğ	"Ï
ListAgentMemoriesResponseg
agent_memories (2,.google.cloud.aiplatform.v1beta1.AgentMemoryBÂöŒÜ"
AUDIT_SIZERagentMemories/
next_page_token (	B  ğ˜çRnextPageToken:òœâ¡internal,v1beta1"€
GetAgentMemoryRequestæ
name (	BÑâAúA'
%aiplatform.googleapis.com/AgentMemory  ğ˜Ì‚ÉÏ™`’óèç‡(not empty($) ?: [REQUIRED_FIELD_MISSING][isValidResourceName($, `AgentMemory`) ?: [FIELD_INVALID] Invalid AgentMemory resource name.€‹±ÍRname^
view (20.google.cloud.aiplatform.v1beta1.AgentMemoryViewBâA  ğ˜çÂöŒÜ"AUDITRview:òœâ¡internal,v1beta1ÀöÄğ	"‹
CreateAgentMemoryRequest¿
parent (	B¦âAúA'%aiplatform.googleapis.com/AgentMemory  ğ˜Ì‚ÉÏ™`’óèç][empty($) or isValidResourceName($, `Agent`) ?: [FIELD_INVALID] Invalid Agent resource name.€‹±ÍRparentŒ
agent_memory (2,.google.cloud.aiplatform.v1beta1.AgentMemoryBºâA’óèç›Lnot empty($.path) ?: [REQUIRED_FIELD_MISSING] agent_memory.path is required.Slen($.path)<=4096 ?: [FIELD_INVALID] agent_memory.path can have at most 4096 bytes.Tnot empty($.category) ?: [REQUIRED_FIELD_MISSING] agent_memory.category is required.
 true ?: [REQUIRED_FIELD_MISSING]ÂöŒÜ"AUDIT_REDACTRagentMemory:òœâ¡internal,v1beta1ÀöÄğ	"‘
BatchCreateAgentMemoriesRequestİ
parent (	BÄâAúA'%aiplatform.googleapis.com/AgentMemory  ğ˜Ì‚ÉÏ™`’óèç{(not empty($) ?: [REQUIRED_FIELD_MISSING]OisValidResourceName($, `Agent`) ?: [FIELD_INVALID] Invalid Agent resource name.€‹±ÍRparentí
requests (29.google.cloud.aiplatform.v1beta1.CreateAgentMemoryRequestB•âA’óèçŠ>not empty($) ?: [REQUIRED_FIELD_MISSING] requests is required.Hlen($)<=100 ?: [FIELD_INVALID] requests can contain at most 100 entries.Rrequests:òœâ¡internal,v1beta1ÀöÄğ	"‘
 BatchCreateAgentMemoriesResponseS
agent_memories (2,.google.cloud.aiplatform.v1beta1.AgentMemoryRagentMemories:òœâ¡internal,v1beta1"å
UpdateAgentMemoryRequestÑ
agent_memory (2,.google.cloud.aiplatform.v1beta1.AgentMemoryBÿâA’óèçÚTnot empty($.name) ?: [REQUIRED_FIELD_MISSING] AgentMemory resource name is required.`isValidResourceName($.name, `AgentMemory`) ?: [FIELD_INVALID] Invalid AgentMemory resource name.
 true ?: [REQUIRED_FIELD_MISSING]ÂöŒÜ"AUDIT_REDACT€‹±ÍRagentMemoryU
update_mask (2.google.protobuf.FieldMaskBâA  ğ˜íÂöŒÜ"AUDITR
updateMask:òœâ¡internal,v1beta1ÀöÄğ	"Ä
DeleteAgentMemoryRequestæ
name (	BÑâAúA'
%aiplatform.googleapis.com/AgentMemory  ğ˜Ì‚ÉÏ™`’óèç‡(not empty($) ?: [REQUIRED_FIELD_MISSING][isValidResourceName($, `AgentMemory`) ?: [FIELD_INVALID] Invalid AgentMemory resource name.€‹±ÍRname
etag (	BâA  ğ˜èRetag:òœâ¡internal,v1beta1ÀöÄğ	"Æ
UndeleteAgentMemoryRequestæ
name (	BÑâAúA'
%aiplatform.googleapis.com/AgentMemory  ğ˜Ì‚ÉÏ™`’óèç‡(not empty($) ?: [REQUIRED_FIELD_MISSING][isValidResourceName($, `AgentMemory`) ?: [FIELD_INVALID] Invalid AgentMemory resource name.€‹±ÍRname
etag (	BâA  ğ˜èRetag:òœâ¡internal,v1beta1ÀöÄğ	"•
ResetAgentMemoriesRequest×
parent (	B¾âAúA!
aiplatform.googleapis.com/Agent  ğ˜Ì‚ÉÏ™`’óèç{(not empty($) ?: [REQUIRED_FIELD_MISSING]OisValidResourceName($, `Agent`) ?: [FIELD_INVALID] Invalid Agent resource name.€‹±ÍRparent:òœâ¡internal,v1beta1ÀöÄğ	"r
ResetAgentMemoriesResponse:
onboarding_started (BâA  ğ˜çRonboardingStarted:òœâ¡internal,v1beta1"Ÿ
ListAgentMemoryRevisionsRequestò
parent (	BÙâAúA/-aiplatform.googleapis.com/AgentMemoryRevision  ğ˜Ì‚ÉÏ™`’óèç‡(not empty($) ?: [REQUIRED_FIELD_MISSING][isValidResourceName($, `AgentMemory`) ?: [FIELD_INVALID] Invalid AgentMemory resource name.€‹±ÍRparentv
	page_size (BYâA  ğ˜ç’óèç;9$>=0 ?: [FIELD_INVALID] Page size should be non-negative.ÂöŒÜ"AUDITRpageSize*

page_token (	BâA  ğ˜çR	pageTokenf
view (28.google.cloud.aiplatform.v1beta1.AgentMemoryRevisionViewBâA  ğ˜ğÂöŒÜ"AUDITRview[
diff_ignores_yaml_frontmatter (BâA  ğ˜ğÂöŒÜ"AUDITRdiffIgnoresYamlFrontmatter:òœâ¡internal,v1beta1ÀöÄğ	"í
 ListAgentMemoryRevisionsResponse~
agent_memory_revisions (24.google.cloud.aiplatform.v1beta1.AgentMemoryRevisionBÂöŒÜ"
AUDIT_SIZERagentMemoryRevisions/
next_page_token (	B  ğ˜çRnextPageToken:òœâ¡internal,v1beta12»
AgentServiceØ
CreateAgent3.google.cloud.aiplatform.v1beta1.CreateAgentRequest.google.longrunning.Operation"ôÊA%
AgentCreateAgentOperationMetadataÚAparent,agent‚Óä“8:agent"//v1beta1/{parent=projects/*/locations/*}/agentsª»Ééë¦
parent'aiplatform.googleapis.com/agents.create+RESOURCE,aiplatform.googleapis.com/Location"AUDIT,AUDIT_TARGET_RESOURCE0:parentRaiplatform.googleapis.com/AgentE
name#aiplatform.googleapis.com/Operationj

EVENT_TIME
CLOCK_TIME"7CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,WRITE_ACTION*
method_typeCREATE2AUTO_CLOUD_IAM:ENABLE_CAIS_INTEGRATION@A      N@Phğ
GetAgent0.google.cloud.aiplatform.v1beta1.GetAgentRequest&.google.cloud.aiplatform.v1beta1.Agent"‰ÚAname‚Óä“1//v1beta1/{name=projects/*/locations/*/agents/*}ª»Éé·u
name$aiplatform.googleapis.com/agents.get(RESOURCE,aiplatform.googleapis.com/Agent"AUDIT,AUDIT_TARGET_RESOURCE0".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAMA      N@Ph‘

ListAgents2.google.cloud.aiplatform.v1beta1.ListAgentsRequest3.google.cloud.aiplatform.v1beta1.ListAgentsResponse"™ÚAparent‚Óä“1//v1beta1/{parent=projects/*/locations/*}/agentsª»ÉéÅ{
parent%aiplatform.googleapis.com/agents.list+RESOURCE,aiplatform.googleapis.com/Location"AUDIT,AUDIT_TARGET_RESOURCE0"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAMA      N@Ph˜
SearchAgents4.google.cloud.aiplatform.v1beta1.SearchAgentsRequest5.google.cloud.aiplatform.v1beta1.SearchAgentsResponse"šÚAparent‚Óä“86/v1beta1/{parent=projects/*/locations/*}/agents:searchª»Éé¿}
parent'aiplatform.googleapis.com/agents.search+RESOURCE,aiplatform.googleapis.com/Location"AUDIT,AUDIT_TARGET_RESOURCE0".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAMA      N@PhÆ
UpdateAgent3.google.cloud.aiplatform.v1beta1.UpdateAgentRequest.google.longrunning.Operation"âÊA%
AgentUpdateAgentOperationMetadataÚAagent,update_mask‚Óä“>:agent25/v1beta1/{agent.name=projects/*/locations/*/agents/*}ª»ÉéÎ~

agent.name'aiplatform.googleapis.com/agents.update(RESOURCE,aiplatform.googleapis.com/Agent"AUDIT,AUDIT_TARGET_RESOURCE0E
name#aiplatform.googleapis.com/Operationj

EVENT_TIME
CLOCK_TIME"CCLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION,WRITE_ACTION*
method_typeUPDATE2AUTO_CLOUD_IAM:ENABLE_CAIS_INTEGRATION@A      N@PhÄ
DeleteAgent3.google.cloud.aiplatform.v1beta1.DeleteAgentRequest.google.longrunning.Operation"àÊA0
google.protobuf.EmptyDeleteOperationMetadataÚAname‚Óä“1*//v1beta1/{name=projects/*/locations/*/agents/*}ª»ÉéÛ’
name'aiplatform.googleapis.com/agents.delete(RESOURCE,aiplatform.googleapis.com/Agent"AUDIT,AUDIT_TARGET_RESOURCE0j

EVENT_TIME
CLOCK_TIMEE
name#aiplatform.googleapis.com/Operationj

EVENT_TIME
CLOCK_TIME";CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION,WRITE_ACTION*
method_typeDELETE2AUTO_CLOUD_IAM:ENABLE_CAIS_INTEGRATION@A      N@Phòœâ¡internal,ui,v1beta1,v1B£
#com.google.cloud.aiplatform.v1beta1BAgentServiceProtoPªGoogle.Cloud.AIPlatform.V1Beta1ÊGoogle\Cloud\AIPlatform\V1beta1ê"Google::Cloud::AIPlatform::V1beta1bproto3
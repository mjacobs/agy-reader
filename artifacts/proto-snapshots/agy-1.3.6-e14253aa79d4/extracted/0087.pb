
7google/cloud/aiplatform/master/prediction_service.protogoogle.cloud.aiplatform.mastergoogle/api/annotations.protogoogle/api/client.protogoogle/api/field_behavior.protogoogle/api/httpbody.protogoogle/api/inclusion.protogoogle/api/policy.protogoogle/api/resource.protogoogle/api/versioning.protogoogle/api/visibility.proto8google/cloud/aiplatform/common/proto_field_options.proto3google/cloud/aiplatform/master/cached_content.proto,google/cloud/aiplatform/master/content.proto0google/cloud/aiplatform/master/explanation.proto.google/cloud/aiplatform/master/operation.proto1google/cloud/aiplatform/master/service_tier.proto)google/cloud/aiplatform/master/tool.proto*google/cloud/aiplatform/master/types.proto3google/cloud/aiplatform/master/usage_metadata.proto4google/cloud/automl/common/proto_field_options.proto0google/cloud/speech/v1p1beta1/cloud_speech.proto#google/longrunning/operations.protogoogle/protobuf/any.protogoogle/protobuf/struct.protogoogle/protobuf/timestamp.proto.net/proto2/contrib/validator/annotations.proto<storage/datapol/annotations/proto/semantic_annotations.proto"‹
PredictRequestk
endpoint (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint9
	instances (2.google.protobuf.ValueBàAR	instances6

parameters (2.google.protobuf.ValueR
parameters^
labels (2:.google.cloud.aiplatform.master.PredictRequest.LabelsEntryB
àA  ð˜ìRlabels9
LabelsEntry
key (	Rkey
value (	Rvalue:8"…
PredictResponse8
predictions (2.google.protobuf.ValueRpredictions@
deployed_model_id (	B  ð˜ÌÂöŒÜ"AUDITRdeployedModelId=
model (	B'àAúA!
aiplatform.googleapis.com/ModelRmodelK
model_version_id (	B!àAòœâ¡internal,ui,v1,v1beta1RmodelVersionId1
model_display_name (	BàARmodelDisplayName7
metadata (2.google.protobuf.ValueBàARmetadata"¶
RawPredictRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint1
	http_body (2.google.api.HttpBodyRhttpBodyJ"¼
StreamRawPredictRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint1
	http_body (2.google.api.HttpBodyRhttpBodyJ"„
DirectPredictRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpointG
inputs (2&.google.cloud.aiplatform.master.TensorB  ð˜ìRinputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parameters"ª
DirectPredictResponseI
outputs (2&.google.cloud.aiplatform.master.TensorB  ð˜ìRoutputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parameters"š
DirectRawPredictRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpoint(
method_name (	B  ð˜ÜR
methodName
input (B	  ð˜ìRinputW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"–
DirectRawPredictResponse!
output (B	  ð˜ìRoutputW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"ë
StreamDirectPredictRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpointJ
inputs (2&.google.cloud.aiplatform.master.TensorB
àA  ð˜ìRinputsK

parameters (2&.google.cloud.aiplatform.master.TensorBàAR
parametersW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"‰
StreamDirectPredictResponseI
outputs (2&.google.cloud.aiplatform.master.TensorB  ð˜ìRoutputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"¦
StreamDirectRawPredictRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpoint+
method_name (	B
àA  ð˜ÜR
methodName"
input (BàA  ð˜ìRinputW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"œ
StreamDirectRawPredictResponse!
output (B	  ð˜ìRoutputW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"à
StreamingPredictRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpointG
inputs (2&.google.cloud.aiplatform.master.TensorB  ð˜ìRinputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"†
StreamingPredictResponseI
outputs (2&.google.cloud.aiplatform.master.TensorB  ð˜ìRoutputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"
StreamingRawPredictRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpoint(
method_name (	B  ð˜ÜR
methodName
input (B	  ð˜ìRinputW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"™
StreamingRawPredictResponse!
output (B	  ð˜ìRoutputW
	send_time (2.google.protobuf.TimestampB  ð˜´úÒä“GOOGLE_INTERNALRsendTime"£
PredictLongRunningRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint9
	instances (2.google.protobuf.ValueBàAR	instances;

parameters (2.google.protobuf.ValueBàAR
parametersi
labels (2E.google.cloud.aiplatform.master.PredictLongRunningRequest.LabelsEntryB
àA  ð˜ìRlabels9
LabelsEntry
key (	Rkey
value (	Rvalue:8"×
RawPredictLongRunningRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint6
	http_body (2.google.api.HttpBodyBàARhttpBody:úÒä“UNIMPLEMENTED"Ð
FetchPredictOperationRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpointF
operation_name (	BàA  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDITRoperationName"ö
InvokeRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint@
deployed_model_id (	B  ð˜ÌÂöŒÜ"AUDITRdeployedModelId1
	http_body (2.google.api.HttpBodyRhttpBody:ÀöÄð	"è
StreamInvokeRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint6
	http_body (2.google.api.HttpBodyBàARhttpBody/
deployed_model_id (	BàARdeployedModelId"I
StreamInvokeResponse1
	http_body (2.google.api.HttpBodyRhttpBody"´
InvokeApiRequesth
endpoint (	BLàAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint6
	http_body (2.google.api.HttpBodyBàARhttpBody"‘
ExplainRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpoint9
	instances (2.google.protobuf.ValueBàAR	instances6

parameters (2.google.protobuf.ValueR
parameters“
explanation_spec_override (27.google.cloud.aiplatform.master.ExplanationSpecOverrideBòœâ¡v1beta1,v1,ui,internalRexplanationSpecOverrideÆ
$concurrent_explanation_spec_override (2U.google.cloud.aiplatform.master.ExplainRequest.ConcurrentExplanationSpecOverrideEntryBàAòœâ¡v1beta1,ui,internalR!concurrentExplanationSpecOverride@
deployed_model_id (	B  ð˜ÌÂöŒÜ"AUDITRdeployedModelId
&ConcurrentExplanationSpecOverrideEntry
key (	RkeyM
value (27.google.cloud.aiplatform.master.ExplanationSpecOverrideRvalue:8"ÿ
ExplainResponseO
explanations (2+.google.cloud.aiplatform.master.ExplanationRexplanations¡
concurrent_explanations (2K.google.cloud.aiplatform.master.ExplainResponse.ConcurrentExplanationsEntryBòœâ¡v1beta1,ui,internalRconcurrentExplanations@
deployed_model_id (	B  ð˜ÌÂöŒÜ"AUDITRdeployedModelId8
predictions (2.google.protobuf.ValueRpredictionsh
ConcurrentExplanationO
explanations (2+.google.cloud.aiplatform.master.ExplanationRexplanations
ConcurrentExplanationsEntry
key (	Rkey[
value (2E.google.cloud.aiplatform.master.ExplainResponse.ConcurrentExplanationRvalue:8"¦
CountTokensRequestk
endpoint (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpointe
model (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodel9
	instances (2.google.protobuf.ValueBàAR	instancesH
contents (2'.google.cloud.aiplatform.master.ContentBàARcontents`
system_instruction (2'.google.cloud.aiplatform.master.ContentBàAH RsystemInstructionˆ?
tools (2$.google.cloud.aiplatform.master.ToolBàARtoolsg
generation_config (20.google.cloud.aiplatform.master.GenerationConfigBàAHRgenerationConfigˆB
_system_instructionB
_generation_config"á
CountTokensResponse!
total_tokens (RtotalTokens:
total_billable_characters (RtotalBillableCharactersk
prompt_tokens_details (22.google.cloud.aiplatform.master.ModalityTokenCountBàARpromptTokensDetails"É
TestGrpcPredictRequest[
endpoint (	B?àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`€‹±ÍRendpoint2
	instances (2.google.protobuf.AnyR	instances:úÒä“VERTEX_PREDICTION_GRPC"q
TestGrpcPredictResponse6
predictions (2.google.protobuf.AnyRpredictions:úÒä“VERTEX_PREDICTION_GRPC"ï
SpeechStreamingRecognizeRequestS
endpoint (	B7àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì€‹±ÍRendpointW

speech_req (28.google.cloud.speech.v1p1beta1.StreamingRecognizeRequestR	speechReq:úÒä“VERTEX_PREDICTION_GRPC"Æ
 SpeechStreamingRecognizeResponse&
endpoint (	B
àA  ð˜ÌRendpointZ
speech_resp (29.google.cloud.speech.v1p1beta1.StreamingRecognizeResponseR
speechResp:úÒä“VERTEX_PREDICTION_GRPC"¬
GenerateContentRequest~
endpoint (	BbàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`úÒä“GOOGLE_INTERNALÂöŒÜ"AUDITRendpointe
model (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelH
contents (2'.google.cloud.aiplatform.master.ContentBàARcontents`
system_instruction (2'.google.cloud.aiplatform.master.ContentBàAH RsystemInstructionˆs
preamble_config (2..google.cloud.aiplatform.master.PreambleConfigBàA  ð˜çòœâ¡
internalRpreambleConfig
cached_content	 (	BfàAúA)
'aiplatform.googleapis.com/CachedContent  ð˜Ì‚ÉÏ™`òœâ¡internal,v1beta1,v1ÂöŒÜ"AUDITRcachedContent?
tools (2$.google.cloud.aiplatform.master.ToolBàARtoolsW
tool_config (2*.google.cloud.aiplatform.master.ToolConfigB
àA  ð˜çR
toolConfigf
labels
 (2B.google.cloud.aiplatform.master.GenerateContentRequest.LabelsEntryB
àA  ð˜ìRlabels[
safety_settings (2-.google.cloud.aiplatform.master.SafetySettingBàARsafetySettingsc
model_armor_config (20.google.cloud.aiplatform.master.ModelArmorConfigBàARmodelArmorConfigb
generation_config (20.google.cloud.aiplatform.master.GenerationConfigBàARgenerationConfigJ
service_account (	B!àA  ð˜ÌúÒä“GOOGLE_INTERNALRserviceAccount?

session_id (	B àA  ð˜üúÒä“LLM_SESSION_IDR	sessionIdw
uniserve_endpoint_override (	B9àA  ð˜ÌúÒä“)'VERTEX_UNISERVE_ENDPOINT_OVERRIDE_USERSRuniserveEndpointOverride‚
1global_endpoint_target_location_hint_testing_only (	BàAúÒä“GOOGLE_INTERNALR+globalEndpointTargetLocationHintTestingOnlyv
service_tier (2+.google.cloud.aiplatform.master.ServiceTierB!àA  ð˜çúÒä“GOOGLE_INTERNALHRserviceTierˆ“
implicit_cache_config (22.google.cloud.aiplatform.master.ContextCacheConfigB&àAúÒä“DURABLE_CACHE_TRUSTED_USERSHRimplicitCacheConfigˆ\

admit_time (2.google.protobuf.TimestampB!àA  ð˜çúÒä“GOOGLE_INTERNALR	admitTimeL
	fail_fast (B/àA  ð˜çúÒä“GOOGLE_INTERNAL,FAIL_FAST_EAPRfailFast9
continuation_token (B
àA  ð˜ìRcontinuationToken„
output_config (2C.google.cloud.aiplatform.master.GenerateContentRequest.OutputConfigBàAúÒä“GOOGLE_INTERNALRoutputConfigF
OutputConfig6
output_uri_prefix (	B
àA  ð˜ìRoutputUriPrefix9
LabelsEntry
key (	Rkey
value (	Rvalue:8:ÀöÄð	B
_system_instructionB
_service_tierB
_implicit_cache_config"ö
GenerateContentResponseN

candidates (2).google.cloud.aiplatform.master.CandidateBàAR
candidates/
model_version (	B
àA  ð˜çRmodelVersionG
create_time (2.google.protobuf.TimestampB
àA  ð˜´R
createTime+
response_id (	B
àA  ð˜çR
responseId{
prompt_feedback (2F.google.cloud.aiplatform.master.GenerateContentResponse.PromptFeedbackB
àA  ð˜çRpromptFeedbackl
usage_metadata (2E.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadataRusageMetadataZ
	send_time
 (2.google.protobuf.TimestampB!àA  ð˜´úÒä“GOOGLE_INTERNALRsendTime·
internal_debug_info (2I.google.cloud.aiplatform.master.GenerateContentResponse.InternalDebugInfoB<àAúÒä“MODEL_OPTIMIZER_TESTERòœâ¡internal,v1beta1,uiRinternalDebugInfož
model_lifecycle_status (2L.google.cloud.aiplatform.master.GenerateContentResponse.ModelLifecycleStatusBàAúÒä“MODEL_LIFECYCLERmodelLifecycleStatus…
logprobs_result (2..google.cloud.aiplatform.master.LogprobsResultB,  ð˜ìúÒä“PROMPT_LOGPROBS_TRUSTED_USERSRlogprobsResult×
PromptFeedbackƒ
block_reason (2T.google.cloud.aiplatform.master.GenerateContentResponse.PromptFeedback.BlockedReasonB
àA  ð˜çRblockReason_
safety_ratings (2,.google.cloud.aiplatform.master.SafetyRatingB
àA  ð˜çRsafetyRatings<
block_reason_message (	B
àA  ð˜çRblockReasonMessage"Ÿ
BlockedReason
BLOCKED_REASON_UNSPECIFIED 

SAFETY	
OTHER
	BLOCKLIST
PROHIBITED_CONTENT
MODEL_ARMOR
IMAGE_SAFETY
	JAILBREAKæ
UsageMetadata,
prompt_token_count (RpromptTokenCount4
candidates_token_count (RcandidatesTokenCount*
total_token_count (RtotalTokenCountA
tool_use_prompt_token_count (BàARtoolUsePromptTokenCount5
thoughts_token_count (BàARthoughtsTokenCount@
cached_content_token_count (BàARcachedContentTokenCountk
prompt_tokens_details	 (22.google.cloud.aiplatform.master.ModalityTokenCountBàARpromptTokensDetailsi
cache_tokens_details
 (22.google.cloud.aiplatform.master.ModalityTokenCountBàARcacheTokensDetailss
candidates_tokens_details (22.google.cloud.aiplatform.master.ModalityTokenCountBàARcandidatesTokensDetails{
tool_use_prompt_tokens_details (22.google.cloud.aiplatform.master.ModalityTokenCountBàARtoolUsePromptTokensDetailsy
traffic_type (2Q.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadata.TrafficTypeBàARtrafficType¦
billable_prompt_usage (2T.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadata.BillablleUsageBàAúÒä“GOOGLE_INTERNALRbillablePromptUsageµ
billable_cached_content_usage (2T.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadata.BillablleUsageBàAúÒä“GOOGLE_INTERNALRbillableCachedContentUsageÀ
BillablleUsage

text_count (R	textCount
image_count (R
imageCount4
video_duration_seconds (RvideoDurationSeconds4
audio_duration_seconds (RaudioDurationSeconds:"™
TrafficType
TRAFFIC_TYPE_UNSPECIFIED 
	ON_DEMAND
ON_DEMAND_PRIORITY
ON_DEMAND_FLEX
ON_DEMAND_OFFPEAK
PROVISIONED_THROUGHPUTJ_
InternalDebugInfo*
routed_model_name (	RroutedModelName:úÒä“MODEL_OPTIMIZER_TESTER•
ModelLifecycleStatus”
model_lifecycle_stage (2`.google.cloud.aiplatform.master.GenerateContentResponse.ModelLifecycleStatus.ModelLifecycleStageRmodelLifecycleStage'
retirement_date (	RretirementDate:
recommended_upgrade_model (	RrecommendedUpgradeModel"€
ModelLifecycleStage%
!MODEL_LIFECYCLE_STAGE_UNSPECIFIED 
EXPERIMENTAL
PREVIEW

STABLE

LEGACY
RETIRED"Ê
CompleteCodeRequeste
model (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelQ
prompts (22.google.cloud.aiplatform.master.CompleteCodePromptBàARpromptsb
completion_config (20.google.cloud.aiplatform.master.CompletionConfigBàARcompletionConfig:úÒä“UNIMPLEMENTED"Î
CompleteCodeResponse[

candidates (26.google.cloud.aiplatform.master.CompleteCodePredictionBàAR
candidates@
deployed_model_id (	B  ð˜ÌÂöŒÜ"AUDITRdeployedModelIdD
model (	B.àAúA!
aiplatform.googleapis.com/Model  ð˜ÌRmodel4
model_version_id (	B
àA  ð˜ÌRmodelVersionId8
model_display_name (	B
àA  ð˜ìRmodelDisplayNameY
metadata (2=.google.cloud.aiplatform.master.CompleteCodeResponse.MetadataRmetadataî
Metadatay
output_token_count (2K.google.cloud.aiplatform.master.CompleteCodeResponse.Metadata.TokenMetadataRoutputTokenCountw
input_token_count (2K.google.cloud.aiplatform.master.CompleteCodeResponse.Metadata.TokenMetadataRinputTokenCountn
TokenMetadata:
total_billable_characters (RtotalBillableCharacters!
total_tokens (RtotalTokens:úÒä“UNIMPLEMENTED"Ñ
AsyncGenerateContentResponsep
session_info (2H.google.cloud.aiplatform.master.AsyncGenerateContentResponse.SessionInfoBàARsessionInfos
state (2Q.google.cloud.aiplatform.master.AsyncGenerateContentResponse.AsyncGenerationStateB
àA  ð˜çRstate}
results (2W.google.cloud.aiplatform.master.AsyncGenerateContentResponse.AsyncGenerateContentResultB
àA  ð˜ìRresults5
SessionInfo&

session_id (	B  ð˜üR	sessionId
AsyncGenerateContentResult
generate_content_response (27.google.cloud.aiplatform.master.GenerateContentResponseB
àA  ð˜ìRgenerateContentResponse"b
AsyncGenerationState&
"ASYNC_GENERATION_STATE_UNSPECIFIED 
PENDING
RUNNING
DONE:òœâ¡
internal"ì
ChatCompletionsRequestk
endpoint (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint=
	http_body (2.google.api.HttpBodyB
àA  ð˜ìRhttpBodyÞ	
	ExtraBodyˆ
model_safety_settings (2T.google.cloud.aiplatform.master.ChatCompletionsRequest.ExtraBody.ModelSafetySettingsRmodelSafetySettingsZ
vertex_rag_store (2..google.cloud.aiplatform.master.VertexRagStoreH RvertexRagStore[
safety_settings (2-.google.cloud.aiplatform.master.SafetySettingBàARsafetySettings,
thought_tag_marker (	RthoughtTagMarker%
cached_content (	RcachedContenth
thinking_config (2?.google.cloud.aiplatform.master.GenerationConfig.ThinkingConfigRthinkingConfig

session_id (	R	sessionId:
tools (2$.google.cloud.aiplatform.master.ToolRtoolsk
media_resolution	 (2@.google.cloud.aiplatform.master.GenerationConfig.MediaResolutionRmediaResolutionC
stream_function_call_arguments
 (RstreamFunctionCallArgumentsk
implicit_cache_config (22.google.cloud.aiplatform.master.ContextCacheConfigBàARimplicitCacheConfig¦
ModelSafetySettings
enabled (BàARenabled›
llama_guard_settings (2g.google.cloud.aiplatform.master.ChatCompletionsRequest.ExtraBody.ModelSafetySettings.LlamaGuardSettingsH RllamaGuardSettings-
LlamaGuardSettings:úÒä“GOOGLE_INTERNAL:úÒä“GOOGLE_INTERNALB

settings:úÒä“GOOGLE_INTERNALB
retrieval_sourceæ
	ExtraPart
thought (HRthoughtˆ+
thought_signature (RthoughtSignatureV
video_metadata (2-.google.cloud.aiplatform.master.VideoMetadataH RvideoMetadata:úÒä“GOOGLE_INTERNALB

metadataB

_thoughtJ©
 PerChoiceResponseExtraPropertiesl
grounding_metadata (21.google.cloud.aiplatform.master.GroundingMetadataB
àA  ð˜çRgroundingMetadata:úÒä“GOOGLE_INTERNAL2
ResponseExtraProperties:úÒä“GOOGLE_INTERNALt
ExtraToolPartJ
parts (24.google.cloud.aiplatform.master.FunctionResponsePartRparts:úÒä“GOOGLE_INTERNAL:ÀöÄð	"¢
PredictLongRunningResponsex
generate_video_response (25.google.cloud.aiplatform.master.GenerateVideoResponseB  ð˜çH RgenerateVideoResponseB

response"
PredictLongRunningMetadata"ï
FetchPredictOperationMetadatar

task_state (2G.google.cloud.aiplatform.master.FetchPredictOperationMetadata.TaskStateB
àA  ð˜çR	taskState"A
	TaskState
TASK_STATE_UNSPECIFIED 
STARTED
PENDING:úÒä“GOOGLE_INTERNAL"é
GenerateVideoResponse/
generated_samples (	BRgeneratedSamplesE
rai_media_filtered_count (B  ð˜çH RraiMediaFilteredCountˆD
rai_media_filtered_reasons (	B  ð˜çRraiMediaFilteredReasons\
videos (2;.google.cloud.aiplatform.master.GenerateVideoResponse.VideoB  ð˜ìRvideos–
Video"
gcs_uri (	B  ð˜ìH RgcsUri;
bytes_base64_encoded (	B  ð˜ìH RbytesBase64Encoded$
	mime_type (	B  ð˜çRmimeTypeB
dataB
_rai_media_filtered_count"Ë
EmbedContentRequestv
model (	B[àA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`’óèç
trueÂöŒÜ"AUDIT€‹±ÍH Rmodelˆ^
content (2'.google.cloud.aiplatform.master.ContentBàA  ð˜ì’óèç
trueHRcontentˆ'
title (	BàA  ð˜ìHRtitleˆu
	task_type (2E.google.cloud.aiplatform.master.EmbedContentRequest.EmbeddingTaskTypeBàA  ð˜ì]HRtaskTypeˆ?
output_dimensionality (BàAHRoutputDimensionalityˆ/
auto_truncate (BàAHRautoTruncateˆ‚
embed_content_config (2F.google.cloud.aiplatform.master.EmbedContentRequest.EmbedContentConfigBàAHRembedContentConfigˆu
service_account	 (	BGàAúA#
!iam.googleapis.com/ServiceAccount  ð˜ÌúÒä“GOOGLE_INTERNALHRserviceAccountˆŠ
labels
 (2?.google.cloud.aiplatform.master.EmbedContentRequest.LabelsEntryB1àA  ð˜ìúÒä“GOOGLE_INTERNALòœâ¡
internalRlabelsû
EmbedContentConfig%
title (	B
àA  ð˜ìH Rtitleˆs
	task_type (2E.google.cloud.aiplatform.master.EmbedContentRequest.EmbeddingTaskTypeB
àA  ð˜ì]HRtaskTypeˆ-
auto_truncate (BàAHRautoTruncateˆ=
output_dimensionality (BàAHRoutputDimensionalityˆ+
document_ocr (BàAHRdocumentOcrˆ>
audio_track_extraction (BàAHRaudioTrackExtractionˆB
_titleB

_task_typeB
_auto_truncateB
_output_dimensionalityB
_document_ocrB
_audio_track_extraction9
LabelsEntry
key (	Rkey
value (	Rvalue:8"ú
EmbeddingTaskType
UNSPECIFIED !
NONEúÒä“GOOGLE_INTERNAL
RETRIEVAL_QUERY
RETRIEVAL_DOCUMENT
SEMANTIC_SIMILARITY
CLASSIFICATION

CLUSTERING
QUESTION_ANSWERING
FACT_VERIFICATION
CODE_RETRIEVAL_QUERY	B
_modelB

_contentB
_titleB

_task_typeB
_output_dimensionalityB
_auto_truncateB
_embed_content_configB
_service_accountJ"“
EmbedContentResponse\
	embedding (2>.google.cloud.aiplatform.master.EmbedContentResponse.EmbeddingR	embeddingT
usage_metadata (2-.google.cloud.aiplatform.master.UsageMetadataRusageMetadata
	truncated (R	truncated#
	Embedding
values (RvaluesJ"ü
BatchEmbedContentsRequeste
model (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodel[
requests (23.google.cloud.aiplatform.master.EmbedContentRequestB
àA  ð˜ìRrequests:òœâ¡internal,v1beta1,v1"µ
BatchEmbedContentsResponsem

embeddings (2D.google.cloud.aiplatform.master.BatchEmbedContentsResponse.EmbeddingB  ð˜çR
embeddings]
usage_metadata (2-.google.cloud.aiplatform.master.UsageMetadataB  ð˜çRusageMetadata,
	Embedding
values (B  ð˜çRvalues:òœâ¡internal,v1beta1,v1"ø
AsyncGenerateContentRequeste
model (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelF
content (2'.google.cloud.aiplatform.master.ContentBàARcontentH
contents (2'.google.cloud.aiplatform.master.ContentBàARcontents?
tools (2$.google.cloud.aiplatform.master.ToolBàARtools)

session_id (	B
àA  ð˜üR	sessionIdb
generation_config (20.google.cloud.aiplatform.master.GenerationConfigBàARgenerationConfig:òœâ¡
internal"í
!StreamAsyncGenerateContentRequeste
model (	BOàA
àAúA$
"aiplatform.googleapis.com/Endpoint  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelE
operation_name (	B  ð˜Ì‚ÉÏ™`ÂöŒÜ"AUDITH RoperationName:òœâ¡
internalB
source"è
%AsyncGenerateContentOperationMetadatac
generic_metadata (28.google.cloud.aiplatform.master.GenericOperationMetadataRgenericMetadata)

session_id (	B
àA  ð˜üR	sessionIdq
usage_metadata (2E.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadataBàARusageMetadata"

model_name (	BàAR	modelName:òœâ¡internal,v1beta1"A
%AsyncGenerateContentOperationResponse:òœâ¡internal,v1beta1"
Response:ƒêAä
"aiplatform.googleapis.com/ResponseQprojects/{project}/locations/{location}/endpoints/{endpoint}/responses/{response}Vprojects/{project}/locations/{location}/publishers/{publisher}/v1/responses/{response}*	responses2responseòœâ¡internal,v1beta1,ui"
GetResponseRequestK
name (	B7àAúA$
"aiplatform.googleapis.com/Response  ð˜Ì€‹±ÍRname:òœâ¡internal,v1beta1,ui,v1"„
DeleteResponseRequestK
name (	B7àAúA$
"aiplatform.googleapis.com/Response  ð˜Ì€‹±ÍRname:òœâ¡internal,v1beta1,ui,v12š~
PredictionServiceš
Predict..google.cloud.aiplatform.master.PredictRequest/.google.cloud.aiplatform.master.PredictResponse"­ÚAendpoint,instances,parameters‚Óä“ :*ZT:*"O/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:predict"E/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:predictª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph€

RawPredict1.google.cloud.aiplatform.master.RawPredictRequest.google.api.HttpBody"¨ÚAendpoint,http_body‚Óä“¦:*ZW:*"R/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:rawPredict"H/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:rawPredictª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phš
StreamRawPredict7.google.cloud.aiplatform.master.StreamRawPredictRequest.google.api.HttpBody"´ÚAendpoint,http_body‚Óä“²:*Z]:*"X/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:streamRawPredict"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:streamRawPredictª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0»
DirectPredict4.google.cloud.aiplatform.master.DirectPredictRequest5.google.cloud.aiplatform.master.DirectPredictResponse"¼‚Óä“P:*"K/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:directPredictª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÇ
DirectRawPredict7.google.cloud.aiplatform.master.DirectRawPredictRequest8.google.cloud.aiplatform.master.DirectRawPredictResponse"¿‚Óä“S:*"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:directRawPredictª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phû
StreamDirectPredict:.google.cloud.aiplatform.master.StreamDirectPredictRequest;.google.cloud.aiplatform.master.StreamDirectPredictResponse"æª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0„
StreamDirectRawPredict=.google.cloud.aiplatform.master.StreamDirectRawPredictRequest>.google.cloud.aiplatform.master.StreamDirectRawPredictResponse"æª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0õ
StreamingPredict7.google.cloud.aiplatform.master.StreamingPredictRequest8.google.cloud.aiplatform.master.StreamingPredictResponse"éª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phˆ(0Ð
ServerStreamingPredict7.google.cloud.aiplatform.master.StreamingPredictRequest8.google.cloud.aiplatform.master.StreamingPredictResponse"À‚Óä“¾:*Zc:*"^/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:serverStreamingPredict"T/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:serverStreamingPredictòœâ¡ui,v1,v1beta1ª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0û
StreamingRawPredict:.google.cloud.aiplatform.master.StreamingRawPredictRequest;.google.cloud.aiplatform.master.StreamingRawPredictResponse"æª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0Ï
PredictLongRunning9.google.cloud.aiplatform.master.PredictLongRunningRequest.google.longrunning.Operation"ÞÚAendpoint,instances,parameters‚Óä“¶:*Z_:*"Z/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:predictLongRunning"P/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:predictLongRunningòœâ¡internal,v1beta1,v1ª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÚ
RawPredictLongRunning<.google.cloud.aiplatform.master.RawPredictLongRunningRequest.google.longrunning.Operation"ãÚAendpoint,http_bodyúÒä“UNIMPLEMENTED‚Óä“¼:*Zb:*"]/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:rawPredictLongRunning"S/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:rawPredictLongRunningòœâ¡
internalª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÕ
FetchPredictOperation<.google.cloud.aiplatform.master.FetchPredictOperationRequest.google.longrunning.Operation"ÞÚAendpoint,operation_name‚Óä“¼:*Zb:*"]/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:fetchPredictOperation"S/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:fetchPredictOperationòœâ¡internal,v1beta1,v1ª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph³	
Invoke-.google.cloud.aiplatform.master.InvokeRequest.google.api.HttpBody"áÚAendpoint‚Óä“Ë:*Zo:*"j/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/deployedModels/{deployed_model_id}/invoke/**Z[:	http_body"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/openapi}/embeddingsZ\:	http_body"O/{$api_version}/{endpoint=projects/*/locations/*/endpoints/openapi}/completionsZZ:	http_body"M/{$api_version}/{endpoint=projects/*/locations/*/endpoints/openapi}/responsesZX:	http_body"K/{$api_version}/{endpoint=projects/*/locations/*/publishers/*}/v1/responsesZ`:	http_body"S/{$api_version}/{endpoint=projects/*/locations/*/publishers/*}/v1/responses/compactZa:	http_body"T/{$api_version}/{endpoint=projects/*/locations/*/endpoints/google}/science/inferenceZV:*"Q/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}/invoke/**"G/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invoke/**òœâ¡internal,v1beta1,ui,v1ª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0”
StreamInvoke3.google.cloud.aiplatform.master.StreamInvokeRequest4.google.cloud.aiplatform.master.StreamInvokeResponse"”úÒä“UNIMPLEMENTEDòœâ¡
internalª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      ¬@Ph(0ù
	InvokeApi0.google.cloud.aiplatform.master.InvokeApiRequest.google.api.HttpBody"¡ÚAendpointúÒä“ GOOGLE_INTERNAL,TRUSTED_TESTER‚Óä“ë:*ZLJ/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**ZL*J/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**"J/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**òœâ¡internal,v1beta1ª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ð
GetResponse2.google.cloud.aiplatform.master.GetResponseRequest.google.api.HttpBody"–ÚAname‚Óä“”ZKI/{$api_version}/{name=projects/*/locations/*/publishers/*/v1/responses/*}E/{$api_version}/{name=projects/*/locations/*/endpoints/*/responses/*}òœâ¡internal,v1beta1,ui,v1ª»ÉéË†
name'aiplatform.googleapis.com/responses.get+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph‡
DeleteResponse5.google.cloud.aiplatform.master.DeleteResponseRequest.google.api.HttpBody"§ÚAname‚Óä“”ZK*I/{$api_version}/{name=projects/*/locations/*/publishers/*/v1/responses/*}*E/{$api_version}/{name=projects/*/locations/*/endpoints/*/responses/*}òœâ¡internal,v1beta1,ui,v1ª»ÉéÜŠ
name*aiplatform.googleapis.com/responses.delete+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b
DATA_WRITE";CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION,WRITE_ACTION2AUTO_CLOUD_IAM@PhÕ
Explain..google.cloud.aiplatform.master.ExplainRequest/.google.cloud.aiplatform.master.ExplainResponse"èÚA/endpoint,instances,parameters,deployed_model_id‚Óä“J:*"E/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:explainª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.explain+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph³
CountTokens2.google.cloud.aiplatform.master.CountTokensRequest3.google.cloud.aiplatform.master.CountTokensResponse"ºÚAendpoint,instances‚Óä“¨:*ZX:*"S/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:countTokens"I/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:countTokensòœâ¡internal,v1beta1ª»ÉéÓŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phø
TestGrpcPredict6.google.cloud.aiplatform.master.TestGrpcPredictRequest7.google.cloud.aiplatform.master.TestGrpcPredictResponse"óúÒä“VERTEX_PREDICTION_GRPC‚Óä“R:*"M/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:testGrpcPredictòœâ¡
internalª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      $@PÃ
SpeechStreamingRecognize?.google.cloud.aiplatform.master.SpeechStreamingRecognizeRequest@.google.cloud.aiplatform.master.SpeechStreamingRecognizeResponse"ŸúÒä“VERTEX_PREDICTION_GRPCòœâ¡
internalª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@À€€€È€€€à(0È
CompleteCode3.google.cloud.aiplatform.master.CompleteCodeRequest4.google.cloud.aiplatform.master.CompleteCodeResponse"ÌÚAendpoint,model,promptsúÒä“UNIMPLEMENTED‚Óä“¤:*ZV:*"Q/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:completeCode"G/{$api_version}/{model=projects/*/locations/*/endpoints/*}:completeCodeòœâ¡
internalª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phª
GenerateContent6.google.cloud.aiplatform.master.GenerateContentRequest7.google.cloud.aiplatform.master.GenerateContentResponse"¥ÚAmodel,contents‚Óä“ª:*ZY:*"T/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:generateContent"J/{$api_version}/{model=projects/*/locations/*/endpoints/*}:generateContentª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph¾
StreamGenerateContent6.google.cloud.aiplatform.master.GenerateContentRequest7.google.cloud.aiplatform.master.GenerateContentResponse"±ÚAmodel,contents‚Óä“¶:*Z_:*"Z/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:streamGenerateContent"P/{$api_version}/{model=projects/*/locations/*/endpoints/*}:streamGenerateContentª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0¿
AsyncGenerateContent;.google.cloud.aiplatform.master.AsyncGenerateContentRequest.google.longrunning.Operation"ÊÊAE
AsyncGenerateContentResponse%AsyncGenerateContentOperationMetadataÚAmodel,contentÚAmodel,content,session_id‚Óä“^:*"Y/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:asyncGenerateContentòœâ¡
internalª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phü
StreamAsyncGenerateContentA.google.cloud.aiplatform.master.StreamAsyncGenerateContentRequest<.google.cloud.aiplatform.master.AsyncGenerateContentResponse"ÚÚAmodel,operation_name‚Óä“À:*Zd:*"_/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:streamAsyncGenerateContent"U/{$api_version}/{model=projects/*/locations/*/endpoints/*}:streamAsyncGenerateContentòœâ¡
internalª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      ¬@Ph0Þ
ChatCompletions6.google.cloud.aiplatform.master.ChatCompletionsRequest.google.api.HttpBody"úÚAendpoint,http_body‚Óä“[:	http_body"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/chat/completionsòœâ¡ui,internal,v1beta1,v1ª»ÉéÛŽ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ï
EmbedContent3.google.cloud.aiplatform.master.EmbedContentRequest4.google.cloud.aiplatform.master.EmbedContentResponse"óÚAmodel,content‚Óä“V:*"Q/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:embedContentòœâ¡v1beta1,internal,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      N@Phˆ
BatchEmbedContents9.google.cloud.aiplatform.master.BatchEmbedContentsRequest:.google.cloud.aiplatform.master.BatchEmbedContentsResponse"úÚAmodel,requests‚Óä“\:*"W/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:batchEmbedContentsòœâ¡internal,v1beta1,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A     À‚@PhBÊŠÕÛÒ
all
"com.google.cloud.aiplatform.masterBPredictionServiceProtoPªGoogle.Cloud.AIPlatform.MasterÊGoogle\Cloud\AIPlatform\Masterê!Google::Cloud::AIPlatform::MasteròOPEN_TO_OPAQUE_HYBRID bproto3

8google/cloud/aiplatform/v1beta1/prediction_service.protogoogle.cloud.aiplatform.v1beta1google/api/annotations.protogoogle/api/client.protogoogle/api/field_behavior.protogoogle/api/httpbody.protogoogle/api/inclusion.protogoogle/api/policy.protogoogle/api/resource.protogoogle/api/versioning.protogoogle/api/visibility.proto8google/cloud/aiplatform/common/proto_field_options.proto4google/cloud/aiplatform/v1beta1/cached_content.proto-google/cloud/aiplatform/v1beta1/content.proto1google/cloud/aiplatform/v1beta1/explanation.proto/google/cloud/aiplatform/v1beta1/operation.proto2google/cloud/aiplatform/v1beta1/service_tier.proto*google/cloud/aiplatform/v1beta1/tool.proto+google/cloud/aiplatform/v1beta1/types.proto4google/cloud/aiplatform/v1beta1/usage_metadata.proto4google/cloud/automl/common/proto_field_options.proto0google/cloud/speech/v1p1beta1/cloud_speech.proto#google/longrunning/operations.protogoogle/protobuf/any.protogoogle/protobuf/struct.protogoogle/protobuf/timestamp.proto.net/proto2/contrib/validator/annotations.proto<storage/datapol/annotations/proto/semantic_annotations.proto"å
PredictRequestk
endpoint (	BO‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint9
	instances (2.google.protobuf.ValueB‡AR	instances6

parameters (2.google.protobuf.ValueR
parameters_
labels (2;.google.cloud.aiplatform.v1beta1.PredictRequest.LabelsEntryB
‡A††òÏRlabels9
LabelsEntry
key (	Rkey
value (	Rvalue:8"Ö
PredictResponse8
predictions (2.google.protobuf.ValueRpredictions@
deployed_model_id (	B††òÃ¬ˆå‹"AUDITRdeployedModelId=
model (	B'‡A˙A!
aiplatform.googleapis.com/ModelRmodelK
model_version_id (	B!‡AÚú‚°internal,ui,v1,v1beta1RmodelVersionId1
model_display_name (	B‡ARmodelDisplayName7
metadata (2.google.protobuf.ValueB‡ARmetadata"∂
RawPredictRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint1
	http_body (2.google.api.HttpBodyRhttpBodyJ"º
StreamRawPredictRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint1
	http_body (2.google.api.HttpBodyRhttpBodyJ"Ü
DirectPredictRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpointH
inputs (2'.google.cloud.aiplatform.v1beta1.TensorB††òÏRinputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
parameters"¨
DirectPredictResponseJ
outputs (2'.google.cloud.aiplatform.v1beta1.TensorB††òÏRoutputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
parameters"ö
DirectRawPredictRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpoint(
method_name (	B††ò‹R
methodName
input (B	††òÏRinputW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"ñ
DirectRawPredictResponse!
output (B	††òÏRoutputW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"Ì
StreamDirectPredictRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpointK
inputs (2'.google.cloud.aiplatform.v1beta1.TensorB
‡A††òÏRinputsL

parameters (2'.google.cloud.aiplatform.v1beta1.TensorB‡AR
parametersW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"ã
StreamDirectPredictResponseJ
outputs (2'.google.cloud.aiplatform.v1beta1.TensorB††òÏRoutputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"¶
StreamDirectRawPredictRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpoint+
method_name (	B
‡A††ò‹R
methodName"
input (B‡A††òÏRinputW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"ú
StreamDirectRawPredictResponse!
output (B	††òÏRoutputW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"‚
StreamingPredictRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpointH
inputs (2'.google.cloud.aiplatform.v1beta1.TensorB††òÏRinputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"à
StreamingPredictResponseJ
outputs (2'.google.cloud.aiplatform.v1beta1.TensorB††òÏRoutputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"ù
StreamingRawPredictRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpoint(
method_name (	B††ò‹R
methodName
input (B	††òÏRinputW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"ô
StreamingRawPredictResponse!
output (B	††òÏRoutputW
	send_time (2.google.protobuf.TimestampB††ò¥˙“‰ìGOOGLE_INTERNALRsendTime"§
PredictLongRunningRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint9
	instances (2.google.protobuf.ValueB‡AR	instances;

parameters (2.google.protobuf.ValueB‡AR
parametersj
labels (2F.google.cloud.aiplatform.v1beta1.PredictLongRunningRequest.LabelsEntryB
‡A††òÏRlabels9
LabelsEntry
key (	Rkey
value (	Rvalue:8"◊
RawPredictLongRunningRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint6
	http_body (2.google.api.HttpBodyB‡ARhttpBody:˙“‰ìUNIMPLEMENTED"–
FetchPredictOperationRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpointF
operation_name (	B‡A††òÃÇ…œô`¬ˆå‹"AUDITRoperationName"ˆ
InvokeRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint@
deployed_model_id (	B††òÃ¬ˆå‹"AUDITRdeployedModelId1
	http_body (2.google.api.HttpBodyRhttpBody:¿ˆƒ	"¿
StreamPublisherInvokeRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint6
	http_body (2.google.api.HttpBodyB‡ARhttpBody"Ë
StreamInvokeRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint6
	http_body (2.google.api.HttpBodyB‡ARhttpBody/
deployed_model_id (	B‡ARdeployedModelId"I
StreamInvokeResponse1
	http_body (2.google.api.HttpBodyRhttpBody"¥
InvokeApiRequesth
endpoint (	BL‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint6
	http_body (2.google.api.HttpBodyB‡ARhttpBody"î
ExplainRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpoint9
	instances (2.google.protobuf.ValueB‡AR	instances6

parameters (2.google.protobuf.ValueR
parametersî
explanation_spec_override (28.google.cloud.aiplatform.v1beta1.ExplanationSpecOverrideBÚú‚°v1beta1,v1,ui,internalRexplanationSpecOverride«
$concurrent_explanation_spec_override (2V.google.cloud.aiplatform.v1beta1.ExplainRequest.ConcurrentExplanationSpecOverrideEntryB‡AÚú‚°v1beta1,ui,internalR!concurrentExplanationSpecOverride@
deployed_model_id (	B††òÃ¬ˆå‹"AUDITRdeployedModelIdé
&ConcurrentExplanationSpecOverrideEntry
key (	RkeyN
value (28.google.cloud.aiplatform.v1beta1.ExplanationSpecOverrideRvalue:8"É
ExplainResponseP
explanations (2,.google.cloud.aiplatform.v1beta1.ExplanationRexplanations¢
concurrent_explanations (2L.google.cloud.aiplatform.v1beta1.ExplainResponse.ConcurrentExplanationsEntryBÚú‚°v1beta1,ui,internalRconcurrentExplanations@
deployed_model_id (	B††òÃ¬ˆå‹"AUDITRdeployedModelId8
predictions (2.google.protobuf.ValueRpredictionsi
ConcurrentExplanationP
explanations (2,.google.cloud.aiplatform.v1beta1.ExplanationRexplanationsë
ConcurrentExplanationsEntry
key (	Rkey\
value (2F.google.cloud.aiplatform.v1beta1.ExplainResponse.ConcurrentExplanationRvalue:8"™
CountTokensRequestk
endpoint (	BO‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpointe
model (	BO‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRmodel9
	instances (2.google.protobuf.ValueB‡AR	instancesI
contents (2(.google.cloud.aiplatform.v1beta1.ContentB‡ARcontentsa
system_instruction (2(.google.cloud.aiplatform.v1beta1.ContentB‡AH RsystemInstructionà@
tools (2%.google.cloud.aiplatform.v1beta1.ToolB‡ARtoolsh
generation_config (21.google.cloud.aiplatform.v1beta1.GenerationConfigB‡AHRgenerationConfigàB
_system_instructionB
_generation_config"‚
CountTokensResponse!
total_tokens (RtotalTokens:
total_billable_characters (RtotalBillableCharactersl
prompt_tokens_details (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountB‡ARpromptTokensDetails"…
TestGrpcPredictRequest[
endpoint (	B?‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`Äã±ÕRendpoint2
	instances (2.google.protobuf.AnyR	instances:˙“‰ìVERTEX_PREDICTION_GRPC"q
TestGrpcPredictResponse6
predictions (2.google.protobuf.AnyRpredictions:˙“‰ìVERTEX_PREDICTION_GRPC"Ô
SpeechStreamingRecognizeRequestS
endpoint (	B7‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÄã±ÕRendpointW

speech_req (28.google.cloud.speech.v1p1beta1.StreamingRecognizeRequestR	speechReq:˙“‰ìVERTEX_PREDICTION_GRPC"∆
 SpeechStreamingRecognizeResponse&
endpoint (	B
‡A††òÃRendpointZ
speech_resp (29.google.cloud.speech.v1p1beta1.StreamingRecognizeResponseR
speechResp:˙“‰ìVERTEX_PREDICTION_GRPC"™
GenerateContentRequest~
endpoint (	Bb‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`˙“‰ìGOOGLE_INTERNAL¬ˆå‹"AUDITRendpointe
model (	BO‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRmodelI
contents (2(.google.cloud.aiplatform.v1beta1.ContentB‡ARcontentsa
system_instruction (2(.google.cloud.aiplatform.v1beta1.ContentB‡AH RsystemInstructionàç
cached_content	 (	Bf‡A˙A)
'aiplatform.googleapis.com/CachedContent††òÃÇ…œô`Úú‚°internal,v1beta1,v1¬ˆå‹"AUDITRcachedContent@
tools (2%.google.cloud.aiplatform.v1beta1.ToolB‡ARtoolsX
tool_config (2+.google.cloud.aiplatform.v1beta1.ToolConfigB
‡A††òÁR
toolConfigg
labels
 (2C.google.cloud.aiplatform.v1beta1.GenerateContentRequest.LabelsEntryB
‡A††òÏRlabels\
safety_settings (2..google.cloud.aiplatform.v1beta1.SafetySettingB‡ARsafetySettingsd
model_armor_config (21.google.cloud.aiplatform.v1beta1.ModelArmorConfigB‡ARmodelArmorConfigc
generation_config (21.google.cloud.aiplatform.v1beta1.GenerationConfigB‡ARgenerationConfigJ
service_account (	B!‡A††òÃ˙“‰ìGOOGLE_INTERNALRserviceAccount?

session_id (	B ‡A††ò¸˙“‰ìLLM_SESSION_IDR	sessionIdw
uniserve_endpoint_override (	B9‡A††òÃ˙“‰ì)'VERTEX_UNISERVE_ENDPOINT_OVERRIDE_USERSRuniserveEndpointOverrideÇ
1global_endpoint_target_location_hint_testing_only (	B‡A˙“‰ìGOOGLE_INTERNALR+globalEndpointTargetLocationHintTestingOnlyw
service_tier (2,.google.cloud.aiplatform.v1beta1.ServiceTierB!‡A††òÁ˙“‰ìGOOGLE_INTERNALHRserviceTieràî
implicit_cache_config (23.google.cloud.aiplatform.v1beta1.ContextCacheConfigB&‡A˙“‰ìDURABLE_CACHE_TRUSTED_USERSHRimplicitCacheConfigà\

admit_time (2.google.protobuf.TimestampB!‡A††òÁ˙“‰ìGOOGLE_INTERNALR	admitTimeL
	fail_fast (B/‡A††òÁ˙“‰ìGOOGLE_INTERNAL,FAIL_FAST_EAPRfailFast9
continuation_token (B
‡A††òÏRcontinuationTokenn
output_config (2D.google.cloud.aiplatform.v1beta1.GenerateContentRequest.OutputConfigB‡ARoutputConfigF
OutputConfig6
output_uri_prefix (	B
‡A††òÏRoutputUriPrefix9
LabelsEntry
key (	Rkey
value (	Rvalue:8:¿ˆƒ	B
_system_instructionB
_service_tierB
_implicit_cache_config"Ü
GenerateContentResponseO

candidates (2*.google.cloud.aiplatform.v1beta1.CandidateB‡AR
candidates/
model_version (	B
‡A††òÁRmodelVersionG
create_time (2.google.protobuf.TimestampB
‡A††ò¥R
createTime+
response_id (	B
‡A††òÁR
responseId|
prompt_feedback (2G.google.cloud.aiplatform.v1beta1.GenerateContentResponse.PromptFeedbackB
‡A††òÁRpromptFeedbackm
usage_metadata (2F.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadataRusageMetadataZ
	send_time
 (2.google.protobuf.TimestampB!‡A††ò¥˙“‰ìGOOGLE_INTERNALRsendTime∏
internal_debug_info (2J.google.cloud.aiplatform.v1beta1.GenerateContentResponse.InternalDebugInfoB<‡A˙“‰ìMODEL_OPTIMIZER_TESTERÚú‚°internal,v1beta1,uiRinternalDebugInfoü
model_lifecycle_status (2M.google.cloud.aiplatform.v1beta1.GenerateContentResponse.ModelLifecycleStatusB‡A˙“‰ìMODEL_LIFECYCLERmodelLifecycleStatusÜ
logprobs_result (2/.google.cloud.aiplatform.v1beta1.LogprobsResultB,††òÏ˙“‰ìPROMPT_LOGPROBS_TRUSTED_USERSRlogprobsResultŸ
PromptFeedbackÑ
block_reason (2U.google.cloud.aiplatform.v1beta1.GenerateContentResponse.PromptFeedback.BlockedReasonB
‡A††òÁRblockReason`
safety_ratings (2-.google.cloud.aiplatform.v1beta1.SafetyRatingB
‡A††òÁRsafetyRatings<
block_reason_message (	B
‡A††òÁRblockReasonMessage"ü
BlockedReason
BLOCKED_REASON_UNSPECIFIED 

SAFETY	
OTHER
	BLOCKLIST
PROHIBITED_CONTENT
MODEL_ARMOR
IMAGE_SAFETY
	JAILBREAKÌ
UsageMetadata,
prompt_token_count (RpromptTokenCount4
candidates_token_count (RcandidatesTokenCount*
total_token_count (RtotalTokenCountA
tool_use_prompt_token_count (B‡ARtoolUsePromptTokenCount5
thoughts_token_count (B‡ARthoughtsTokenCount@
cached_content_token_count (B‡ARcachedContentTokenCountl
prompt_tokens_details	 (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountB‡ARpromptTokensDetailsj
cache_tokens_details
 (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountB‡ARcacheTokensDetailst
candidates_tokens_details (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountB‡ARcandidatesTokensDetails|
tool_use_prompt_tokens_details (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountB‡ARtoolUsePromptTokensDetailsz
traffic_type (2R.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadata.TrafficTypeB‡ARtrafficTypeß
billable_prompt_usage (2U.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadata.BillablleUsageB‡A˙“‰ìGOOGLE_INTERNALRbillablePromptUsage∂
billable_cached_content_usage (2U.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadata.BillablleUsageB‡A˙“‰ìGOOGLE_INTERNALRbillableCachedContentUsage¿
BillablleUsage

text_count (R	textCount
image_count (R
imageCount4
video_duration_seconds (RvideoDurationSeconds4
audio_duration_seconds (RaudioDurationSeconds:"ô
TrafficType
TRAFFIC_TYPE_UNSPECIFIED 
	ON_DEMAND
ON_DEMAND_PRIORITY
ON_DEMAND_FLEX
ON_DEMAND_OFFPEAK
PROVISIONED_THROUGHPUTJ_
InternalDebugInfo*
routed_model_name (	RroutedModelName:˙“‰ìMODEL_OPTIMIZER_TESTERñ
ModelLifecycleStatusï
model_lifecycle_stage (2a.google.cloud.aiplatform.v1beta1.GenerateContentResponse.ModelLifecycleStatus.ModelLifecycleStageRmodelLifecycleStage'
retirement_date (	RretirementDate:
recommended_upgrade_model (	RrecommendedUpgradeModel"Ä
ModelLifecycleStage%
!MODEL_LIFECYCLE_STAGE_UNSPECIFIED 
EXPERIMENTAL
PREVIEW

STABLE

LEGACY
RETIRED"Ã
CompleteCodeRequeste
model (	BO‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRmodelR
prompts (23.google.cloud.aiplatform.v1beta1.CompleteCodePromptB‡ARpromptsc
completion_config (21.google.cloud.aiplatform.v1beta1.CompletionConfigB‡ARcompletionConfig:˙“‰ìUNIMPLEMENTED"“
CompleteCodeResponse\

candidates (27.google.cloud.aiplatform.v1beta1.CompleteCodePredictionB‡AR
candidates@
deployed_model_id (	B††òÃ¬ˆå‹"AUDITRdeployedModelIdD
model (	B.‡A˙A!
aiplatform.googleapis.com/Model††òÃRmodel4
model_version_id (	B
‡A††òÃRmodelVersionId8
model_display_name (	B
‡A††òÏRmodelDisplayNameZ
metadata (2>.google.cloud.aiplatform.v1beta1.CompleteCodeResponse.MetadataRmetadata
Metadataz
output_token_count (2L.google.cloud.aiplatform.v1beta1.CompleteCodeResponse.Metadata.TokenMetadataRoutputTokenCountx
input_token_count (2L.google.cloud.aiplatform.v1beta1.CompleteCodeResponse.Metadata.TokenMetadataRinputTokenCountn
TokenMetadata:
total_billable_characters (RtotalBillableCharacters!
total_tokens (RtotalTokens:˙“‰ìUNIMPLEMENTED"˜
ChatCompletionsRequestk
endpoint (	BO‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRendpoint=
	http_body (2.google.api.HttpBodyB
‡A††òÏRhttpBodyÊ	
	ExtraBody[
vertex_rag_store (2/.google.cloud.aiplatform.v1beta1.VertexRagStoreH RvertexRagStoreâ
model_safety_settings (2U.google.cloud.aiplatform.v1beta1.ChatCompletionsRequest.ExtraBody.ModelSafetySettingsRmodelSafetySettings\
safety_settings (2..google.cloud.aiplatform.v1beta1.SafetySettingB‡ARsafetySettings,
thought_tag_marker (	RthoughtTagMarker%
cached_content (	RcachedContenti
thinking_config (2@.google.cloud.aiplatform.v1beta1.GenerationConfig.ThinkingConfigRthinkingConfig

session_id (	R	sessionId;
tools (2%.google.cloud.aiplatform.v1beta1.ToolRtoolsl
media_resolution	 (2A.google.cloud.aiplatform.v1beta1.GenerationConfig.MediaResolutionRmediaResolutionC
stream_function_call_arguments
 (RstreamFunctionCallArgumentsl
implicit_cache_config (23.google.cloud.aiplatform.v1beta1.ContextCacheConfigB‡ARimplicitCacheConfigß
ModelSafetySettingsú
llama_guard_settings (2h.google.cloud.aiplatform.v1beta1.ChatCompletionsRequest.ExtraBody.ModelSafetySettings.LlamaGuardSettingsH RllamaGuardSettings
enabled (B‡ARenabled-
LlamaGuardSettings:˙“‰ìGOOGLE_INTERNAL:˙“‰ìGOOGLE_INTERNALB

settings:˙“‰ìGOOGLE_INTERNALB
retrieval_sourceÁ
	ExtraPartW
video_metadata (2..google.cloud.aiplatform.v1beta1.VideoMetadataH RvideoMetadata
thought (HRthoughtà+
thought_signature (RthoughtSignature:˙“‰ìGOOGLE_INTERNALB

metadataB

_thoughtJ™
 PerChoiceResponseExtraPropertiesm
grounding_metadata (22.google.cloud.aiplatform.v1beta1.GroundingMetadataB
‡A††òÁRgroundingMetadata:˙“‰ìGOOGLE_INTERNAL2
ResponseExtraProperties:˙“‰ìGOOGLE_INTERNALu
ExtraToolPartK
parts (25.google.cloud.aiplatform.v1beta1.FunctionResponsePartRparts:˙“‰ìGOOGLE_INTERNAL:¿ˆƒ	"£
PredictLongRunningResponsey
generate_video_response (26.google.cloud.aiplatform.v1beta1.GenerateVideoResponseB††òÁH RgenerateVideoResponseB

response"
PredictLongRunningMetadata"
FetchPredictOperationMetadatas

task_state (2H.google.cloud.aiplatform.v1beta1.FetchPredictOperationMetadata.TaskStateB
‡A††òÁR	taskState"A
	TaskState
TASK_STATE_UNSPECIFIED 
STARTED
PENDING:˙“‰ìGOOGLE_INTERNAL"Í
GenerateVideoResponse/
generated_samples (	BRgeneratedSamplesE
rai_media_filtered_count (B††òÁH RraiMediaFilteredCountàD
rai_media_filtered_reasons (	B††òÁRraiMediaFilteredReasons]
videos (2<.google.cloud.aiplatform.v1beta1.GenerateVideoResponse.VideoB††òÏRvideosñ
Video"
gcs_uri (	B††òÏH RgcsUri;
bytes_base64_encoded (	B††òÏH RbytesBase64Encoded$
	mime_type (	B††òÁRmimeTypeB
dataB
_rai_media_filtered_count"á
EmbedContentRequestv
model (	B[‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`íÛËÁ
true¬ˆå‹"AUDITÄã±ÕH Rmodelà_
content (2(.google.cloud.aiplatform.v1beta1.ContentB‡A††òÏíÛËÁ
trueHRcontentà'
title (	B‡A††òÏHRtitleàv
	task_type (2F.google.cloud.aiplatform.v1beta1.EmbedContentRequest.EmbeddingTaskTypeB‡A††òÏ]HRtaskTypeà?
output_dimensionality (B‡AHRoutputDimensionalityà/
auto_truncate (B‡AHRautoTruncateàÉ
embed_content_config (2G.google.cloud.aiplatform.v1beta1.EmbedContentRequest.EmbedContentConfigB‡AHRembedContentConfigàu
service_account	 (	BG‡A˙A#
!iam.googleapis.com/ServiceAccount††òÃ˙“‰ìGOOGLE_INTERNALHRserviceAccountà¸
EmbedContentConfig%
title (	B
‡A††òÏH Rtitleàt
	task_type (2F.google.cloud.aiplatform.v1beta1.EmbedContentRequest.EmbeddingTaskTypeB
‡A††òÏ]HRtaskTypeà-
auto_truncate (B‡AHRautoTruncateà=
output_dimensionality (B‡AHRoutputDimensionalityà+
document_ocr (B‡AHRdocumentOcrà>
audio_track_extraction (B‡AHRaudioTrackExtractionàB
_titleB

_task_typeB
_auto_truncateB
_output_dimensionalityB
_document_ocrB
_audio_track_extraction"˙
EmbeddingTaskType
UNSPECIFIED !
NONE˙“‰ìGOOGLE_INTERNAL
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
_service_accountJ"ï
EmbedContentResponse]
	embedding (2?.google.cloud.aiplatform.v1beta1.EmbedContentResponse.EmbeddingR	embeddingU
usage_metadata (2..google.cloud.aiplatform.v1beta1.UsageMetadataRusageMetadata
	truncated (R	truncated#
	Embedding
values (RvaluesJ"˝
BatchEmbedContentsRequeste
model (	BO‡A
‡A˙A$
"aiplatform.googleapis.com/Endpoint††òÃÇ…œô`¬ˆå‹"AUDITÄã±ÕRmodel\
requests (24.google.cloud.aiplatform.v1beta1.EmbedContentRequestB
‡A††òÏRrequests:Úú‚°internal,v1beta1,v1"∑
BatchEmbedContentsResponsen

embeddings (2E.google.cloud.aiplatform.v1beta1.BatchEmbedContentsResponse.EmbeddingB††òÁR
embeddings^
usage_metadata (2..google.cloud.aiplatform.v1beta1.UsageMetadataB††òÁRusageMetadata,
	Embedding
values (B††òÁRvalues:Úú‚°internal,v1beta1,v1"Í
%AsyncGenerateContentOperationMetadatad
generic_metadata (29.google.cloud.aiplatform.v1beta1.GenericOperationMetadataRgenericMetadata)

session_id (	B
‡A††ò¸R	sessionIdr
usage_metadata (2F.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadataB‡ARusageMetadata"

model_name (	B‡AR	modelName:Úú‚°internal,v1beta1"A
%AsyncGenerateContentOperationResponse:Úú‚°internal,v1beta1"ê
Response:ÉÍA‰
"aiplatform.googleapis.com/ResponseQprojects/{project}/locations/{location}/endpoints/{endpoint}/responses/{response}Vprojects/{project}/locations/{location}/publishers/{publisher}/v1/responses/{response}*	responses2responseÚú‚°internal,v1beta1,ui"Å
GetResponseRequestK
name (	B7‡A˙A$
"aiplatform.googleapis.com/Response††òÃÄã±ÕRname:Úú‚°internal,v1beta1,ui,v1"Ñ
DeleteResponseRequestK
name (	B7‡A˙A$
"aiplatform.googleapis.com/Response††òÃÄã±ÕRname:Úú‚°internal,v1beta1,ui,v12”b
PredictionServiceé
Predict/.google.cloud.aiplatform.v1beta1.PredictRequest0.google.cloud.aiplatform.v1beta1.PredictResponse"ü⁄Aendpoint,instances,parametersÇ”‰ìí:*ZM:*"H/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:predict">/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:predict™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÛ

RawPredict2.google.cloud.aiplatform.v1beta1.RawPredictRequest.google.api.HttpBody"ö⁄Aendpoint,http_bodyÇ”‰ìò:*ZP:*"K/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:rawPredict"A/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:rawPredict™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phç
StreamRawPredict8.google.cloud.aiplatform.v1beta1.StreamRawPredictRequest.google.api.HttpBody"¶⁄Aendpoint,http_bodyÇ”‰ì§:*ZV:*"Q/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:streamRawPredict"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:streamRawPredict™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0∂
DirectPredict5.google.cloud.aiplatform.v1beta1.DirectPredictRequest6.google.cloud.aiplatform.v1beta1.DirectPredictResponse"µÇ”‰ìI:*"D/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:directPredict™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph¬
DirectRawPredict8.google.cloud.aiplatform.v1beta1.DirectRawPredictRequest9.google.cloud.aiplatform.v1beta1.DirectRawPredictResponse"∏Ç”‰ìL:*"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:directRawPredict™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph˝
StreamDirectPredict;.google.cloud.aiplatform.v1beta1.StreamDirectPredictRequest<.google.cloud.aiplatform.v1beta1.StreamDirectPredictResponse"Ê™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0Ü
StreamDirectRawPredict>.google.cloud.aiplatform.v1beta1.StreamDirectRawPredictRequest?.google.cloud.aiplatform.v1beta1.StreamDirectRawPredictResponse"Ê™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0˜
StreamingPredict8.google.cloud.aiplatform.v1beta1.StreamingPredictRequest9.google.cloud.aiplatform.v1beta1.StreamingPredictResponse"È™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phà(0ƒ
ServerStreamingPredict8.google.cloud.aiplatform.v1beta1.StreamingPredictRequest9.google.cloud.aiplatform.v1beta1.StreamingPredictResponse"≤Ç”‰ì∞:*Z\:*"W/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:serverStreamingPredict"M/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:serverStreamingPredictÚú‚°ui,v1,v1beta1™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0˝
StreamingRawPredict;.google.cloud.aiplatform.v1beta1.StreamingRawPredictRequest<.google.cloud.aiplatform.v1beta1.StreamingRawPredictResponse"Ê™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0¬
PredictLongRunning:.google.cloud.aiplatform.v1beta1.PredictLongRunningRequest.google.longrunning.Operation"–⁄Aendpoint,instances,parametersÇ”‰ì®:*ZX:*"S/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:predictLongRunning"I/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:predictLongRunningÚú‚°internal,v1beta1,v1™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph»
FetchPredictOperation=.google.cloud.aiplatform.v1beta1.FetchPredictOperationRequest.google.longrunning.Operation"–⁄Aendpoint,operation_nameÇ”‰ìÆ:*Z[:*"V/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:fetchPredictOperation"L/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:fetchPredictOperationÚú‚°internal,v1beta1,v1™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phı
Invoke..google.cloud.aiplatform.v1beta1.InvokeRequest.google.api.HttpBody"¢⁄AendpointÇ”‰ìå:*Zh:*"c/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/deployedModels/{deployed_model_id}/invoke/**ZT:	http_body"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/openapi}/embeddingsZU:	http_body"H/v1beta1/{endpoint=projects/*/locations/*/endpoints/openapi}/completionsZS:	http_body"F/v1beta1/{endpoint=projects/*/locations/*/endpoints/openapi}/responsesZQ:	http_body"D/v1beta1/{endpoint=projects/*/locations/*/publishers/*}/v1/responsesZY:	http_body"L/v1beta1/{endpoint=projects/*/locations/*/publishers/*}/v1/responses/compactZZ:	http_body"M/v1beta1/{endpoint=projects/*/locations/*/endpoints/google}/science/inferenceZO:*"J/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}/invoke/**"@/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invoke/**Úú‚°internal,v1beta1,ui,v1™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0Í
StreamPublisherInvoke=.google.cloud.aiplatform.v1beta1.StreamPublisherInvokeRequest.google.api.HttpBody"˜⁄AendpointÇ”‰ì\r$HIDE_FROM_DISCOVERYD/v1beta1/{endpoint=projects/*/locations/*/publishers/*}/v1/responsesÚú‚°internal,v1,v1beta1™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      ¨@Ph(0Â
	InvokeApi1.google.cloud.aiplatform.v1beta1.InvokeApiRequest.google.api.HttpBody"å⁄Aendpoint˙“‰ì GOOGLE_INTERNAL,TRUSTED_TESTERÇ”‰ì÷:*ZEC/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**ZE*C/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**"C/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**Úú‚°internal,v1beta1™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0„
GetResponse3.google.cloud.aiplatform.v1beta1.GetResponseRequest.google.api.HttpBody"à⁄AnameÇ”‰ìÜZDB/v1beta1/{name=projects/*/locations/*/publishers/*/v1/responses/*}>/v1beta1/{name=projects/*/locations/*/endpoints/*/responses/*}Úú‚°internal,v1beta1,ui,v1™ª…ÈÀÜ
name'aiplatform.googleapis.com/responses.get+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph˙
DeleteResponse6.google.cloud.aiplatform.v1beta1.DeleteResponseRequest.google.api.HttpBody"ô⁄AnameÇ”‰ìÜZD*B/v1beta1/{name=projects/*/locations/*/publishers/*/v1/responses/*}*>/v1beta1/{name=projects/*/locations/*/endpoints/*/responses/*}Úú‚°internal,v1beta1,ui,v1™ª…È‹ä
name*aiplatform.googleapis.com/responses.delete+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b
DATA_WRITE";CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION,WRITE_ACTION2AUTO_CLOUD_IAM@Ph–
Explain/.google.cloud.aiplatform.v1beta1.ExplainRequest0.google.cloud.aiplatform.v1beta1.ExplainResponse"·⁄A/endpoint,instances,parameters,deployed_model_idÇ”‰ìC:*">/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:explain™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.explain+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phß
CountTokens3.google.cloud.aiplatform.v1beta1.CountTokensRequest4.google.cloud.aiplatform.v1beta1.CountTokensResponse"¨⁄Aendpoint,instancesÇ”‰ìö:*ZQ:*"L/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:countTokens"B/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:countTokensÚú‚°internal,v1beta1™ª…È”é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phû
GenerateContent7.google.cloud.aiplatform.v1beta1.GenerateContentRequest8.google.cloud.aiplatform.v1beta1.GenerateContentResponse"ó⁄Amodel,contentsÇ”‰ìú:*ZR:*"M/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:generateContent"C/v1beta1/{model=projects/*/locations/*/endpoints/*}:generateContent™ª…Èÿã
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph≤
StreamGenerateContent7.google.cloud.aiplatform.v1beta1.GenerateContentRequest8.google.cloud.aiplatform.v1beta1.GenerateContentResponse"£⁄Amodel,contentsÇ”‰ì®:*ZX:*"S/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:streamGenerateContent"I/v1beta1/{model=projects/*/locations/*/endpoints/*}:streamGenerateContent™ª…Èÿã
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ÿ
ChatCompletions7.google.cloud.aiplatform.v1beta1.ChatCompletionsRequest.google.api.HttpBody"Û⁄Aendpoint,http_bodyÇ”‰ìT:	http_body"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/chat/completionsÚú‚°ui,internal,v1beta1,v1™ª…È€é
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0Í
EmbedContent4.google.cloud.aiplatform.v1beta1.EmbedContentRequest5.google.cloud.aiplatform.v1beta1.EmbedContentResponse"Ï⁄Amodel,contentÇ”‰ìO:*"J/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:embedContentÚú‚°v1beta1,internal,v1™ª…Èÿã
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      N@PhÉ
BatchEmbedContents:.google.cloud.aiplatform.v1beta1.BatchEmbedContentsRequest;.google.cloud.aiplatform.v1beta1.BatchEmbedContentsResponse"Û⁄Amodel,requestsÇ”‰ìU:*"P/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:batchEmbedContentsÚú‚°internal,v1beta1,v1™ª…Èÿã
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A     ¿Ç@PhBŒä’€“
all
#com.google.cloud.aiplatform.v1beta1BPredictionServiceProtoP™Google.Cloud.AIPlatform.V1Beta1 Google\Cloud\AIPlatform\V1beta1Í"Google::Cloud::AIPlatform::V1beta1ÚOPEN_TO_OPAQUE_HYBRID†bproto3
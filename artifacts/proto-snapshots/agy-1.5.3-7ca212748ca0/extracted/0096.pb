
7google/cloud/aiplatform/master/prediction_service.protogoogle.cloud.aiplatform.mastergoogle/api/annotations.protogoogle/api/client.protogoogle/api/field_behavior.protogoogle/api/httpbody.protogoogle/api/inclusion.protogoogle/api/policy.protogoogle/api/resource.protogoogle/api/versioning.protogoogle/api/visibility.proto8google/cloud/aiplatform/common/proto_field_options.proto3google/cloud/aiplatform/master/cached_content.proto,google/cloud/aiplatform/master/content.proto0google/cloud/aiplatform/master/explanation.proto.google/cloud/aiplatform/master/operation.proto1google/cloud/aiplatform/master/service_tier.proto)google/cloud/aiplatform/master/tool.proto*google/cloud/aiplatform/master/types.proto3google/cloud/aiplatform/master/usage_metadata.proto4google/cloud/automl/common/proto_field_options.proto0google/cloud/speech/v1p1beta1/cloud_speech.proto#google/longrunning/operations.protogoogle/protobuf/any.protogoogle/protobuf/struct.protogoogle/protobuf/timestamp.proto.net/proto2/contrib/validator/annotations.proto<storage/datapol/annotations/proto/semantic_annotations.proto"Œ
PredictRequestj
endpoint (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint:
	instances (2.google.protobuf.ValueBâAR	instances6

parameters (2.google.protobuf.ValueR
parameters_
labels (2:.google.cloud.aiplatform.master.PredictRequest.LabelsEntryBâA  ğ˜ìRlabels9
LabelsEntry
key (	Rkey
value (	Rvalue:8"‰
PredictResponse8
predictions (2.google.protobuf.ValueRpredictions@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelId>
model (	B(âAúA!
aiplatform.googleapis.com/ModelRmodelL
model_version_id (	B"âAòœâ¡internal,ui,v1,v1beta1RmodelVersionId2
model_display_name (	BâARmodelDisplayName8
metadata (2.google.protobuf.ValueBâARmetadata"·
RawPredictRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint1
	http_body (2.google.api.HttpBodyRhttpBodyJ"½
StreamRawPredictRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint1
	http_body (2.google.api.HttpBodyRhttpBodyJ"…
DirectPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpointG
inputs (2&.google.cloud.aiplatform.master.TensorB  ğ˜ìRinputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parameters"ª
DirectPredictResponseI
outputs (2&.google.cloud.aiplatform.master.TensorB  ğ˜ìRoutputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parameters"›
DirectRawPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpoint(
method_name (	B  ğ˜ÜR
methodName
input (B	  ğ˜ìRinputW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"–
DirectRawPredictResponse!
output (B	  ğ˜ìRoutputW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"î
StreamDirectPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpointK
inputs (2&.google.cloud.aiplatform.master.TensorBâA  ğ˜ìRinputsL

parameters (2&.google.cloud.aiplatform.master.TensorBâAR
parametersW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"‰
StreamDirectPredictResponseI
outputs (2&.google.cloud.aiplatform.master.TensorB  ğ˜ìRoutputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"©
StreamDirectRawPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpoint,
method_name (	BâA  ğ˜ÜR
methodName#
input (BâA  ğ˜ìRinputW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"œ
StreamDirectRawPredictResponse!
output (B	  ğ˜ìRoutputW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"á
StreamingPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpointG
inputs (2&.google.cloud.aiplatform.master.TensorB  ğ˜ìRinputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"†
StreamingPredictResponseI
outputs (2&.google.cloud.aiplatform.master.TensorB  ğ˜ìRoutputsF

parameters (2&.google.cloud.aiplatform.master.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"
StreamingRawPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpoint(
method_name (	B  ğ˜ÜR
methodName
input (B	  ğ˜ìRinputW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"™
StreamingRawPredictResponse!
output (B	  ğ˜ìRoutputW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"§
PredictLongRunningRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint:
	instances (2.google.protobuf.ValueBâAR	instances<

parameters (2.google.protobuf.ValueBâAR
parametersj
labels (2E.google.cloud.aiplatform.master.PredictLongRunningRequest.LabelsEntryBâA  ğ˜ìRlabels9
LabelsEntry
key (	Rkey
value (	Rvalue:8"Ù
RawPredictLongRunningRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint7
	http_body (2.google.api.HttpBodyBâARhttpBody:úÒä“UNIMPLEMENTED"Ò
FetchPredictOperationRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpointG
operation_name (	B âA  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDITRoperationName"÷
InvokeRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelId1
	http_body (2.google.api.HttpBodyRhttpBody:ÀöÄğ	"Â
StreamPublisherInvokeRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint7
	http_body (2.google.api.HttpBodyBâARhttpBody"ë
StreamInvokeRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint7
	http_body (2.google.api.HttpBodyBâARhttpBody0
deployed_model_id (	BâARdeployedModelId"I
StreamInvokeResponse1
	http_body (2.google.api.HttpBodyRhttpBody"¶
InvokeApiRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint7
	http_body (2.google.api.HttpBodyBâARhttpBody"”
ExplainRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpoint:
	instances (2.google.protobuf.ValueBâAR	instances6

parameters (2.google.protobuf.ValueR
parameters“
explanation_spec_override (27.google.cloud.aiplatform.master.ExplanationSpecOverrideBòœâ¡v1beta1,v1,ui,internalRexplanationSpecOverrideÇ
$concurrent_explanation_spec_override (2U.google.cloud.aiplatform.master.ExplainRequest.ConcurrentExplanationSpecOverrideEntryBâAòœâ¡v1beta1,ui,internalR!concurrentExplanationSpecOverride@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelId
&ConcurrentExplanationSpecOverrideEntry
key (	RkeyM
value (27.google.cloud.aiplatform.master.ExplanationSpecOverrideRvalue:8"ÿ
ExplainResponseO
explanations (2+.google.cloud.aiplatform.master.ExplanationRexplanations¡
concurrent_explanations (2K.google.cloud.aiplatform.master.ExplainResponse.ConcurrentExplanationsEntryBòœâ¡v1beta1,ui,internalRconcurrentExplanations@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelId8
predictions (2.google.protobuf.ValueRpredictionsh
ConcurrentExplanationO
explanations (2+.google.cloud.aiplatform.master.ExplanationRexplanations
ConcurrentExplanationsEntry
key (	Rkey[
value (2E.google.cloud.aiplatform.master.ExplainResponse.ConcurrentExplanationRvalue:8"©
CountTokensRequestj
endpoint (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpointd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodel:
	instances (2.google.protobuf.ValueBâAR	instancesI
contents (2'.google.cloud.aiplatform.master.ContentBâARcontentsa
system_instruction (2'.google.cloud.aiplatform.master.ContentBâAH RsystemInstructionˆ@
tools (2$.google.cloud.aiplatform.master.ToolBâARtoolsh
generation_config (20.google.cloud.aiplatform.master.GenerationConfigBâAHRgenerationConfigˆB
_system_instructionB
_generation_config"â
CountTokensResponse!
total_tokens (RtotalTokens:
total_billable_characters (RtotalBillableCharactersl
prompt_tokens_details (22.google.cloud.aiplatform.master.ModalityTokenCountBâARpromptTokensDetails"Ê
TestGrpcPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpoint2
	instances (2.google.protobuf.AnyR	instances:úÒä“VERTEX_PREDICTION_GRPC"q
TestGrpcPredictResponse6
predictions (2.google.protobuf.AnyRpredictions:úÒä“VERTEX_PREDICTION_GRPC"ğ
SpeechStreamingRecognizeRequestT
endpoint (	B8âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì€‹±ÍRendpointW

speech_req (28.google.cloud.speech.v1p1beta1.StreamingRecognizeRequestR	speechReq:úÒä“VERTEX_PREDICTION_GRPC"Ç
 SpeechStreamingRecognizeResponse'
endpoint (	BâA  ğ˜ÌRendpointZ
speech_resp (29.google.cloud.speech.v1p1beta1.StreamingRecognizeResponseR
speechResp:úÒä“VERTEX_PREDICTION_GRPC"§
GenerateContentRequest}
endpoint (	BaâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`úÒä“GOOGLE_INTERNALÂöŒÜ"AUDITRendpointd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelI
contents (2'.google.cloud.aiplatform.master.ContentBâARcontentsa
system_instruction (2'.google.cloud.aiplatform.master.ContentBâAH RsystemInstructionˆt
preamble_config (2..google.cloud.aiplatform.master.PreambleConfigBâA  ğ˜çòœâ¡
internalRpreambleConfig
cached_content	 (	BgâAúA)
'aiplatform.googleapis.com/CachedContent  ğ˜Ì‚ÉÏ™`òœâ¡internal,v1beta1,v1ÂöŒÜ"AUDITRcachedContent@
tools (2$.google.cloud.aiplatform.master.ToolBâARtoolsX
tool_config (2*.google.cloud.aiplatform.master.ToolConfigBâA  ğ˜çR
toolConfigg
labels
 (2B.google.cloud.aiplatform.master.GenerateContentRequest.LabelsEntryBâA  ğ˜ìRlabels\
safety_settings (2-.google.cloud.aiplatform.master.SafetySettingBâARsafetySettingsd
model_armor_config (20.google.cloud.aiplatform.master.ModelArmorConfigBâARmodelArmorConfigc
generation_config (20.google.cloud.aiplatform.master.GenerationConfigBâARgenerationConfigK
service_account (	B"âA  ğ˜ÌúÒä“GOOGLE_INTERNALRserviceAccount@

session_id (	B!âA  ğ˜üúÒä“LLM_SESSION_IDR	sessionIdx
uniserve_endpoint_override (	B:âA  ğ˜ÌúÒä“)'VERTEX_UNISERVE_ENDPOINT_OVERRIDE_USERSRuniserveEndpointOverrideƒ
1global_endpoint_target_location_hint_testing_only (	BâAúÒä“GOOGLE_INTERNALR+globalEndpointTargetLocationHintTestingOnlyw
service_tier (2+.google.cloud.aiplatform.master.ServiceTierB"âA  ğ˜çúÒä“GOOGLE_INTERNALHRserviceTierˆ”
implicit_cache_config (22.google.cloud.aiplatform.master.ContextCacheConfigB'âAúÒä“DURABLE_CACHE_TRUSTED_USERSHRimplicitCacheConfigˆ]

admit_time (2.google.protobuf.TimestampB"âA  ğ˜çúÒä“GOOGLE_INTERNALR	admitTimeM
	fail_fast (B0âA  ğ˜çúÒä“GOOGLE_INTERNAL,FAIL_FAST_EAPRfailFast:
continuation_token (BâA  ğ˜ìRcontinuationTokenn
output_config (2C.google.cloud.aiplatform.master.GenerateContentRequest.OutputConfigBâARoutputConfigG
OutputConfig7
output_uri_prefix (	BâA  ğ˜ìRoutputUriPrefix9
LabelsEntry
key (	Rkey
value (	Rvalue:8:ÀöÄğ	B
_system_instructionB
_service_tierB
_implicit_cache_config"‹
GenerateContentResponseO

candidates (2).google.cloud.aiplatform.master.CandidateBâAR
candidates0
model_version (	BâA  ğ˜çRmodelVersionH
create_time (2.google.protobuf.TimestampBâA  ğ˜´R
createTime,
response_id (	BâA  ğ˜çR
responseId|
prompt_feedback (2F.google.cloud.aiplatform.master.GenerateContentResponse.PromptFeedbackBâA  ğ˜çRpromptFeedbackl
usage_metadata (2E.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadataRusageMetadata[
	send_time
 (2.google.protobuf.TimestampB"âA  ğ˜´úÒä“GOOGLE_INTERNALRsendTime¸
internal_debug_info (2I.google.cloud.aiplatform.master.GenerateContentResponse.InternalDebugInfoB=âAúÒä“MODEL_OPTIMIZER_TESTERòœâ¡internal,v1beta1,uiRinternalDebugInfoŸ
model_lifecycle_status (2L.google.cloud.aiplatform.master.GenerateContentResponse.ModelLifecycleStatusBâAúÒä“MODEL_LIFECYCLERmodelLifecycleStatus…
logprobs_result (2..google.cloud.aiplatform.master.LogprobsResultB,  ğ˜ìúÒä“PROMPT_LOGPROBS_TRUSTED_USERSRlogprobsResultÚ
PromptFeedback„
block_reason (2T.google.cloud.aiplatform.master.GenerateContentResponse.PromptFeedback.BlockedReasonBâA  ğ˜çRblockReason`
safety_ratings (2,.google.cloud.aiplatform.master.SafetyRatingBâA  ğ˜çRsafetyRatings=
block_reason_message (	BâA  ğ˜çRblockReasonMessage"Ÿ
BlockedReason
BLOCKED_REASON_UNSPECIFIED 

SAFETY	
OTHER
	BLOCKLIST
PROHIBITED_CONTENT
MODEL_ARMOR
IMAGE_SAFETY
	JAILBREAKğ
UsageMetadata,
prompt_token_count (RpromptTokenCount4
candidates_token_count (RcandidatesTokenCount*
total_token_count (RtotalTokenCountB
tool_use_prompt_token_count (BâARtoolUsePromptTokenCount6
thoughts_token_count (BâARthoughtsTokenCountA
cached_content_token_count (BâARcachedContentTokenCountl
prompt_tokens_details	 (22.google.cloud.aiplatform.master.ModalityTokenCountBâARpromptTokensDetailsj
cache_tokens_details
 (22.google.cloud.aiplatform.master.ModalityTokenCountBâARcacheTokensDetailst
candidates_tokens_details (22.google.cloud.aiplatform.master.ModalityTokenCountBâARcandidatesTokensDetails|
tool_use_prompt_tokens_details (22.google.cloud.aiplatform.master.ModalityTokenCountBâARtoolUsePromptTokensDetailsz
traffic_type (2Q.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadata.TrafficTypeBâARtrafficType§
billable_prompt_usage (2T.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadata.BillablleUsageBâAúÒä“GOOGLE_INTERNALRbillablePromptUsage¶
billable_cached_content_usage (2T.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadata.BillablleUsageBâAúÒä“GOOGLE_INTERNALRbillableCachedContentUsageÀ
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
RETIRED"Ë
CompleteCodeRequestd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelR
prompts (22.google.cloud.aiplatform.master.CompleteCodePromptBâARpromptsc
completion_config (20.google.cloud.aiplatform.master.CompletionConfigBâARcompletionConfig:úÒä“UNIMPLEMENTED"Ò
CompleteCodeResponse\

candidates (26.google.cloud.aiplatform.master.CompleteCodePredictionBâAR
candidates@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelIdE
model (	B/âAúA!
aiplatform.googleapis.com/Model  ğ˜ÌRmodel5
model_version_id (	BâA  ğ˜ÌRmodelVersionId9
model_display_name (	BâA  ğ˜ìRmodelDisplayNameY
metadata (2=.google.cloud.aiplatform.master.CompleteCodeResponse.MetadataRmetadataî
Metadatay
output_token_count (2K.google.cloud.aiplatform.master.CompleteCodeResponse.Metadata.TokenMetadataRoutputTokenCountw
input_token_count (2K.google.cloud.aiplatform.master.CompleteCodeResponse.Metadata.TokenMetadataRinputTokenCountn
TokenMetadata:
total_billable_characters (RtotalBillableCharacters!
total_tokens (RtotalTokens:úÒä“UNIMPLEMENTED"Ö
AsyncGenerateContentResponseq
session_info (2H.google.cloud.aiplatform.master.AsyncGenerateContentResponse.SessionInfoBâARsessionInfot
state (2Q.google.cloud.aiplatform.master.AsyncGenerateContentResponse.AsyncGenerationStateBâA  ğ˜çRstate~
results (2W.google.cloud.aiplatform.master.AsyncGenerateContentResponse.AsyncGenerateContentResultBâA  ğ˜ìRresults5
SessionInfo&

session_id (	B  ğ˜üR	sessionIdŸ
AsyncGenerateContentResult€
generate_content_response (27.google.cloud.aiplatform.master.GenerateContentResponseBâA  ğ˜ìRgenerateContentResponse"b
AsyncGenerationState&
"ASYNC_GENERATION_STATE_UNSPECIFIED 
PENDING
RUNNING
DONE:òœâ¡
internal"ğ
ChatCompletionsRequestj
endpoint (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint>
	http_body (2.google.api.HttpBodyBâA  ğ˜ìRhttpBodyá	
	ExtraBodyˆ
model_safety_settings (2T.google.cloud.aiplatform.master.ChatCompletionsRequest.ExtraBody.ModelSafetySettingsRmodelSafetySettingsZ
vertex_rag_store (2..google.cloud.aiplatform.master.VertexRagStoreH RvertexRagStore\
safety_settings (2-.google.cloud.aiplatform.master.SafetySettingBâARsafetySettings,
thought_tag_marker (	RthoughtTagMarker%
cached_content (	RcachedContenth
thinking_config (2?.google.cloud.aiplatform.master.GenerationConfig.ThinkingConfigRthinkingConfig

session_id (	R	sessionId:
tools (2$.google.cloud.aiplatform.master.ToolRtoolsk
media_resolution	 (2@.google.cloud.aiplatform.master.GenerationConfig.MediaResolutionRmediaResolutionC
stream_function_call_arguments
 (RstreamFunctionCallArgumentsl
implicit_cache_config (22.google.cloud.aiplatform.master.ContextCacheConfigBâARimplicitCacheConfig§
ModelSafetySettings
enabled (BâARenabled›
llama_guard_settings (2g.google.cloud.aiplatform.master.ChatCompletionsRequest.ExtraBody.ModelSafetySettings.LlamaGuardSettingsH RllamaGuardSettings-
LlamaGuardSettings:úÒä“GOOGLE_INTERNAL:úÒä“GOOGLE_INTERNALB

settings:úÒä“GOOGLE_INTERNALB
retrieval_sourceæ
	ExtraPart
thought (HRthoughtˆ+
thought_signature (RthoughtSignatureV
video_metadata (2-.google.cloud.aiplatform.master.VideoMetadataH RvideoMetadata:úÒä“GOOGLE_INTERNALB

metadataB

_thoughtJª
 PerChoiceResponseExtraPropertiesm
grounding_metadata (21.google.cloud.aiplatform.master.GroundingMetadataBâA  ğ˜çRgroundingMetadata:úÒä“GOOGLE_INTERNAL2
ResponseExtraProperties:úÒä“GOOGLE_INTERNALt
ExtraToolPartJ
parts (24.google.cloud.aiplatform.master.FunctionResponsePartRparts:úÒä“GOOGLE_INTERNAL:ÀöÄğ	"¢
PredictLongRunningResponsex
generate_video_response (25.google.cloud.aiplatform.master.GenerateVideoResponseB  ğ˜çH RgenerateVideoResponseB

response"
PredictLongRunningMetadata"ğ
FetchPredictOperationMetadatas

task_state (2G.google.cloud.aiplatform.master.FetchPredictOperationMetadata.TaskStateBâA  ğ˜çR	taskState"A
	TaskState
TASK_STATE_UNSPECIFIED 
STARTED
PENDING:úÒä“GOOGLE_INTERNAL"é
GenerateVideoResponse/
generated_samples (	BRgeneratedSamplesE
rai_media_filtered_count (B  ğ˜çH RraiMediaFilteredCountˆD
rai_media_filtered_reasons (	B  ğ˜çRraiMediaFilteredReasons\
videos (2;.google.cloud.aiplatform.master.GenerateVideoResponse.VideoB  ğ˜ìRvideos–
Video"
gcs_uri (	B  ğ˜ìH RgcsUri;
bytes_base64_encoded (	B  ğ˜ìH RbytesBase64Encoded$
	mime_type (	B  ğ˜çRmimeTypeB
dataB
_rai_media_filtered_count"Ø
EmbedContentRequestu
model (	BZâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`’óèç
trueÂöŒÜ"AUDIT€‹±ÍH Rmodelˆ_
content (2'.google.cloud.aiplatform.master.ContentBâA  ğ˜ì’óèç
trueHRcontentˆ(
title (	BâA  ğ˜ìHRtitleˆv
	task_type (2E.google.cloud.aiplatform.master.EmbedContentRequest.EmbeddingTaskTypeBâA  ğ˜ì]HRtaskTypeˆ@
output_dimensionality (BâAHRoutputDimensionalityˆ0
auto_truncate (BâAHRautoTruncateˆƒ
embed_content_config (2F.google.cloud.aiplatform.master.EmbedContentRequest.EmbedContentConfigBâAHRembedContentConfigˆv
service_account	 (	BHâAúA#
!iam.googleapis.com/ServiceAccount  ğ˜ÌúÒä“GOOGLE_INTERNALHRserviceAccountˆ‹
labels
 (2?.google.cloud.aiplatform.master.EmbedContentRequest.LabelsEntryB2âA  ğ˜ìúÒä“GOOGLE_INTERNALòœâ¡
internalRlabels
EmbedContentConfig&
title (	BâA  ğ˜ìH Rtitleˆt
	task_type (2E.google.cloud.aiplatform.master.EmbedContentRequest.EmbeddingTaskTypeBâA  ğ˜ì]HRtaskTypeˆ.
auto_truncate (BâAHRautoTruncateˆ>
output_dimensionality (BâAHRoutputDimensionalityˆ,
document_ocr (BâAHRdocumentOcrˆ?
audio_track_extraction (BâAHRaudioTrackExtractionˆB
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
BatchEmbedContentsRequestd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodel\
requests (23.google.cloud.aiplatform.master.EmbedContentRequestBâA  ğ˜ìRrequests:òœâ¡internal,v1beta1,v1"µ
BatchEmbedContentsResponsem

embeddings (2D.google.cloud.aiplatform.master.BatchEmbedContentsResponse.EmbeddingB  ğ˜çR
embeddings]
usage_metadata (2-.google.cloud.aiplatform.master.UsageMetadataB  ğ˜çRusageMetadata,
	Embedding
values (B  ğ˜çRvalues:òœâ¡internal,v1beta1,v1"ü
AsyncGenerateContentRequestd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelG
content (2'.google.cloud.aiplatform.master.ContentBâARcontentI
contents (2'.google.cloud.aiplatform.master.ContentBâARcontents@
tools (2$.google.cloud.aiplatform.master.ToolBâARtools*

session_id (	BâA  ğ˜üR	sessionIdc
generation_config (20.google.cloud.aiplatform.master.GenerationConfigBâARgenerationConfig:òœâ¡
internal"ì
!StreamAsyncGenerateContentRequestd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelE
operation_name (	B  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDITH RoperationName:òœâ¡
internalB
source"ë
%AsyncGenerateContentOperationMetadatac
generic_metadata (28.google.cloud.aiplatform.master.GenericOperationMetadataRgenericMetadata*

session_id (	BâA  ğ˜üR	sessionIdr
usage_metadata (2E.google.cloud.aiplatform.master.GenerateContentResponse.UsageMetadataBâARusageMetadata#

model_name (	BâAR	modelName:òœâ¡internal,v1beta1"A
%AsyncGenerateContentOperationResponse:òœâ¡internal,v1beta1"
Response:ƒêAä
"aiplatform.googleapis.com/ResponseQprojects/{project}/locations/{location}/endpoints/{endpoint}/responses/{response}Vprojects/{project}/locations/{location}/publishers/{publisher}/v1/responses/{response}*	responses2responseòœâ¡internal,v1beta1,ui"¿
	MaasModel:±êA’
#aiplatform.googleapis.com/MaasModelTprojects/{project}/locations/{location}/publishers/{publisher}/v1/models/{maasModel}*
maasModels2	maasModelòœâ¡internal,v1beta1,ui"‚
GetResponseRequestL
name (	B8âAúA$
"aiplatform.googleapis.com/Response  ğ˜Ì€‹±ÍRname:òœâ¡internal,v1beta1,ui,v1"…
DeleteResponseRequestL
name (	B8âAúA$
"aiplatform.googleapis.com/Response  ğ˜Ì€‹±ÍRname:òœâ¡internal,v1beta1,ui,v1"¸
DetectContentRequestu
model (	BZâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`’óèç
trueÂöŒÜ"AUDIT€‹±ÍH Rmodelˆ_
content (2'.google.cloud.aiplatform.master.ContentBâA  ğ˜ì’óèç
trueHRcontentˆ:2úÒä“GOOGLE_INTERNALòœâ¡internal,v1beta1,v1B
_modelB

_content"ò
DetectContentResponse
ai_generated_likelihood (2K.google.cloud.aiplatform.master.DetectContentResponse.AiGeneratedLikelihoodBâA  ğ˜çRaiGeneratedLikelihooda
c2pa (2@.google.cloud.aiplatform.master.DetectContentResponse.C2paResultBâA  ğ˜ìRc2pa,
response_id (	BâA  ğ˜çR
responseId=
model_version (	BâA  ğ˜çÂöŒÜ"AUDITRmodelVersionÃ

C2paResulto
outcome (2H.google.cloud.aiplatform.master.DetectContentResponse.C2paResult.OutcomeBâA  ğ˜çRoutcomer
manifest (2I.google.cloud.aiplatform.master.DetectContentResponse.C2paResult.ManifestBâA  ğ˜ìRmanifest×
ManifestŠ
creation_types (2V.google.cloud.aiplatform.master.DetectContentResponse.C2paResult.Manifest.CreationTypeBâA  ğ˜çRcreationTypesw
actions (2P.google.cloud.aiplatform.master.DetectContentResponse.C2paResult.Manifest.ActionBâA  ğ˜çRactions<
signer_organization (	BâA  ğ˜ÌRsignerOrganization9
signer_common_name (	BâA  ğ˜ÌRsignerCommonName4
claim_generator (	BâA  ğ˜ÌRclaimGeneratorD
	sign_time (2.google.protobuf.TimestampBâA  ğ˜´RsignTime"»
CreationType
CREATION_TYPE_UNSPECIFIED 
MEDIA_OF_UNKNOWN_PROVENANCE

BLANK_FILE
REAL_LIFE_CAPTURE9
5REAL_LIFE_CAPTURE_WITH_IN_LINE_NON_GENAI_ENHANCEMENTS
REAL_LIFE_STITCHED_CAPTURE
PARTLY_GENAI_CREATION
FULLY_GENAI_CREATION
NON_GENAI_DIGITAL_CREATION/
+COMPOSITION_WHICH_MAY_CONTAIN_GENAI_CONTENT	/
+DIGITAL_ART_WHICH_MAY_CONTAIN_GENAI_CONTENT
2
.SCREEN_CAPTURE_WHICH_MAY_CONTAIN_GENAI_CONTENT"‘
Action
ACTION_UNSPECIFIED $
 ADJUSTED_COLOR_USING_GENAI_TOOLS(
$ADJUSTED_COLOR_USING_NON_GENAI_TOOLS8
4ADJUSTED_COLOR_USING_TOOLS_POSSIBLY_POWERED_BY_GENAI
CHANGED_ORIENTATION
CROPPED_USING_GENAI_TOOLS!
CROPPED_USING_NON_GENAI_TOOLS1
-CROPPED_USING_TOOLS_POSSIBLY_POWERED_BY_GENAI%
!DELETED_CONTENT_USING_GENAI_TOOLS)
%DELETED_CONTENT_USING_NON_GENAI_TOOLS	9
5DELETED_CONTENT_USING_TOOLS_POSSIBLY_POWERED_BY_GENAI

EDITED_USING_GENAI_TOOLS 
EDITED_USING_NON_GENAI_TOOLS0
,EDITED_USING_TOOLS_POSSIBLY_POWERED_BY_GENAI
EDITED_METADATA%
!ENHANCED_WITHOUT_CHANGING_CONTENT 
STYLIZED_USING_GENAI_FILTERS$
 STYLIZED_USING_NON_GENAI_FILTERS4
0STYLIZED_USING_FILTERS_POSSIBLY_POWERED_BY_GENAI
REMOVED_SENSITIVE_METADATA"v
Outcome
OUTCOME_UNSPECIFIED 
VALID_MANIFEST
NOT_DETECTED
INVALID_MANIFEST
UNSUPPORTED_FORMAT"|
AiGeneratedLikelihood'
#AI_GENERATED_LIKELIHOOD_UNSPECIFIED 
VERY_LOW
LOW

MEDIUM
HIGH
	VERY_HIGH:2úÒä“GOOGLE_INTERNALòœâ¡internal,v1beta1,v12š†
PredictionServiceš
Predict..google.cloud.aiplatform.master.PredictRequest/.google.cloud.aiplatform.master.PredictResponse"­ÚAendpoint,instances,parameters‚Óä“ :*ZT:*"O/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:predict"E/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:predictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph€

RawPredict1.google.cloud.aiplatform.master.RawPredictRequest.google.api.HttpBody"¨ÚAendpoint,http_body‚Óä“¦:*ZW:*"R/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:rawPredict"H/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:rawPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phš
StreamRawPredict7.google.cloud.aiplatform.master.StreamRawPredictRequest.google.api.HttpBody"´ÚAendpoint,http_body‚Óä“²:*Z]:*"X/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:streamRawPredict"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:streamRawPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0»
DirectPredict4.google.cloud.aiplatform.master.DirectPredictRequest5.google.cloud.aiplatform.master.DirectPredictResponse"¼‚Óä“P:*"K/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:directPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÇ
DirectRawPredict7.google.cloud.aiplatform.master.DirectRawPredictRequest8.google.cloud.aiplatform.master.DirectRawPredictResponse"¿‚Óä“S:*"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:directRawPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phû
StreamDirectPredict:.google.cloud.aiplatform.master.StreamDirectPredictRequest;.google.cloud.aiplatform.master.StreamDirectPredictResponse"æª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0„
StreamDirectRawPredict=.google.cloud.aiplatform.master.StreamDirectRawPredictRequest>.google.cloud.aiplatform.master.StreamDirectRawPredictResponse"æª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0õ
StreamingPredict7.google.cloud.aiplatform.master.StreamingPredictRequest8.google.cloud.aiplatform.master.StreamingPredictResponse"éª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phˆ(0Ğ
ServerStreamingPredict7.google.cloud.aiplatform.master.StreamingPredictRequest8.google.cloud.aiplatform.master.StreamingPredictResponse"À‚Óä“¾:*Zc:*"^/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:serverStreamingPredict"T/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:serverStreamingPredictòœâ¡ui,v1,v1beta1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0û
StreamingRawPredict:.google.cloud.aiplatform.master.StreamingRawPredictRequest;.google.cloud.aiplatform.master.StreamingRawPredictResponse"æª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0Ï
PredictLongRunning9.google.cloud.aiplatform.master.PredictLongRunningRequest.google.longrunning.Operation"ŞÚAendpoint,instances,parameters‚Óä“¶:*Z_:*"Z/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:predictLongRunning"P/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:predictLongRunningòœâ¡internal,v1beta1,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÚ
RawPredictLongRunning<.google.cloud.aiplatform.master.RawPredictLongRunningRequest.google.longrunning.Operation"ãÚAendpoint,http_bodyúÒä“UNIMPLEMENTED‚Óä“¼:*Zb:*"]/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:rawPredictLongRunning"S/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:rawPredictLongRunningòœâ¡
internalª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÕ
FetchPredictOperation<.google.cloud.aiplatform.master.FetchPredictOperationRequest.google.longrunning.Operation"ŞÚAendpoint,operation_name‚Óä“¼:*Zb:*"]/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:fetchPredictOperation"S/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:fetchPredictOperationòœâ¡internal,v1beta1,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph³	
Invoke-.google.cloud.aiplatform.master.InvokeRequest.google.api.HttpBody"áÚAendpoint‚Óä“Ë:*Zo:*"j/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/deployedModels/{deployed_model_id}/invoke/**Z[:	http_body"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/openapi}/embeddingsZ\:	http_body"O/{$api_version}/{endpoint=projects/*/locations/*/endpoints/openapi}/completionsZZ:	http_body"M/{$api_version}/{endpoint=projects/*/locations/*/endpoints/openapi}/responsesZX:	http_body"K/{$api_version}/{endpoint=projects/*/locations/*/publishers/*}/v1/responsesZ`:	http_body"S/{$api_version}/{endpoint=projects/*/locations/*/publishers/*}/v1/responses/compactZa:	http_body"T/{$api_version}/{endpoint=projects/*/locations/*/endpoints/google}/science/inferenceZV:*"Q/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}/invoke/**"G/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invoke/**òœâ¡internal,v1beta1,ui,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ğ
StreamPublisherInvoke<.google.cloud.aiplatform.master.StreamPublisherInvokeRequest.google.api.HttpBody"şÚAendpoint‚Óä“cr$HIDE_FROM_DISCOVERYK/{$api_version}/{endpoint=projects/*/locations/*/publishers/*}/v1/responsesòœâ¡internal,v1,v1beta1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      ¬@Ph(0”
StreamInvoke3.google.cloud.aiplatform.master.StreamInvokeRequest4.google.cloud.aiplatform.master.StreamInvokeResponse"”úÒä“UNIMPLEMENTEDòœâ¡
internalª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      ¬@Ph(0ù
	InvokeApi0.google.cloud.aiplatform.master.InvokeApiRequest.google.api.HttpBody"¡ÚAendpointúÒä“ GOOGLE_INTERNAL,TRUSTED_TESTER‚Óä“ë:*ZLJ/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**ZL*J/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**"J/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**òœâ¡internal,v1beta1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ğ
GetResponse2.google.cloud.aiplatform.master.GetResponseRequest.google.api.HttpBody"–ÚAname‚Óä“”ZKI/{$api_version}/{name=projects/*/locations/*/publishers/*/v1/responses/*}E/{$api_version}/{name=projects/*/locations/*/endpoints/*/responses/*}òœâ¡internal,v1beta1,ui,v1ª»ÉéË†
name'aiplatform.googleapis.com/responses.get+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph‡
DeleteResponse5.google.cloud.aiplatform.master.DeleteResponseRequest.google.api.HttpBody"§ÚAname‚Óä“”ZK*I/{$api_version}/{name=projects/*/locations/*/publishers/*/v1/responses/*}*E/{$api_version}/{name=projects/*/locations/*/endpoints/*/responses/*}òœâ¡internal,v1beta1,ui,v1ª»ÉéÜŠ
name*aiplatform.googleapis.com/responses.delete+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b
DATA_WRITE";CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION,WRITE_ACTION2AUTO_CLOUD_IAM@PhÕ
Explain..google.cloud.aiplatform.master.ExplainRequest/.google.cloud.aiplatform.master.ExplainResponse"èÚA/endpoint,instances,parameters,deployed_model_id‚Óä“J:*"E/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:explainª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.explain+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph³
CountTokens2.google.cloud.aiplatform.master.CountTokensRequest3.google.cloud.aiplatform.master.CountTokensResponse"ºÚAendpoint,instances‚Óä“¨:*ZX:*"S/{$api_version}/{endpoint=projects/*/locations/*/publishers/*/models/*}:countTokens"I/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:countTokensòœâ¡internal,v1beta1ª»ÉéÓ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phø
TestGrpcPredict6.google.cloud.aiplatform.master.TestGrpcPredictRequest7.google.cloud.aiplatform.master.TestGrpcPredictResponse"óúÒä“VERTEX_PREDICTION_GRPC‚Óä“R:*"M/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}:testGrpcPredictòœâ¡
internalª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      $@PÃ
SpeechStreamingRecognize?.google.cloud.aiplatform.master.SpeechStreamingRecognizeRequest@.google.cloud.aiplatform.master.SpeechStreamingRecognizeResponse"ŸúÒä“VERTEX_PREDICTION_GRPCòœâ¡
internalª»ÉéÛ
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
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      ¬@Ph0Ş
ChatCompletions6.google.cloud.aiplatform.master.ChatCompletionsRequest.google.api.HttpBody"úÚAendpoint,http_body‚Óä“[:	http_body"N/{$api_version}/{endpoint=projects/*/locations/*/endpoints/*}/chat/completionsòœâ¡ui,internal,v1beta1,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0Š
DetectContent4.google.cloud.aiplatform.master.DetectContentRequest5.google.cloud.aiplatform.master.DetectContentResponse"‹ÚAmodel,contentúÒä“GOOGLE_INTERNAL‚Óä“W:*"R/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:detectContentòœâ¡internal,v1beta1,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      N@Phï
EmbedContent3.google.cloud.aiplatform.master.EmbedContentRequest4.google.cloud.aiplatform.master.EmbedContentResponse"óÚAmodel,content‚Óä“V:*"Q/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:embedContentòœâ¡v1beta1,internal,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      N@Phˆ
BatchEmbedContents9.google.cloud.aiplatform.master.BatchEmbedContentsRequest:.google.cloud.aiplatform.master.BatchEmbedContentsResponse"úÚAmodel,requests‚Óä“\:*"W/{$api_version}/{model=projects/*/locations/*/publishers/*/models/*}:batchEmbedContentsòœâ¡internal,v1beta1,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A     À‚@PhBÊŠÕÛÒ
all
"com.google.cloud.aiplatform.masterBPredictionServiceProtoPªGoogle.Cloud.AIPlatform.MasterÊGoogle\Cloud\AIPlatform\Masterê!Google::Cloud::AIPlatform::MasteròOPEN_TO_OPAQUE_HYBRID bproto3

8google/cloud/aiplatform/v1beta1/prediction_service.protogoogle.cloud.aiplatform.v1beta1google/api/annotations.protogoogle/api/client.protogoogle/api/field_behavior.protogoogle/api/httpbody.protogoogle/api/inclusion.protogoogle/api/policy.protogoogle/api/resource.protogoogle/api/versioning.protogoogle/api/visibility.proto8google/cloud/aiplatform/common/proto_field_options.proto4google/cloud/aiplatform/v1beta1/cached_content.proto-google/cloud/aiplatform/v1beta1/content.proto1google/cloud/aiplatform/v1beta1/explanation.proto/google/cloud/aiplatform/v1beta1/operation.proto2google/cloud/aiplatform/v1beta1/service_tier.proto*google/cloud/aiplatform/v1beta1/tool.proto+google/cloud/aiplatform/v1beta1/types.proto4google/cloud/aiplatform/v1beta1/usage_metadata.proto4google/cloud/automl/common/proto_field_options.proto0google/cloud/speech/v1p1beta1/cloud_speech.proto#google/longrunning/operations.protogoogle/protobuf/any.protogoogle/protobuf/struct.protogoogle/protobuf/timestamp.proto.net/proto2/contrib/validator/annotations.proto<storage/datapol/annotations/proto/semantic_annotations.proto"
PredictRequestj
endpoint (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint:
	instances (2.google.protobuf.ValueBâAR	instances6

parameters (2.google.protobuf.ValueR
parameters`
labels (2;.google.cloud.aiplatform.v1beta1.PredictRequest.LabelsEntryBâA  ğ˜ìRlabels9
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
	http_body (2.google.api.HttpBodyRhttpBodyJ"‡
DirectPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpointH
inputs (2'.google.cloud.aiplatform.v1beta1.TensorB  ğ˜ìRinputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
parameters"¬
DirectPredictResponseJ
outputs (2'.google.cloud.aiplatform.v1beta1.TensorB  ğ˜ìRoutputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
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
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"ğ
StreamDirectPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpointL
inputs (2'.google.cloud.aiplatform.v1beta1.TensorBâA  ğ˜ìRinputsM

parameters (2'.google.cloud.aiplatform.v1beta1.TensorBâAR
parametersW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"‹
StreamDirectPredictResponseJ
outputs (2'.google.cloud.aiplatform.v1beta1.TensorB  ğ˜ìRoutputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
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
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"ã
StreamingPredictRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpointH
inputs (2'.google.cloud.aiplatform.v1beta1.TensorB  ğ˜ìRinputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
parametersW
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"ˆ
StreamingPredictResponseJ
outputs (2'.google.cloud.aiplatform.v1beta1.TensorB  ğ˜ìRoutputsG

parameters (2'.google.cloud.aiplatform.v1beta1.TensorR
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
	send_time (2.google.protobuf.TimestampB  ğ˜´úÒä“GOOGLE_INTERNALRsendTime"¨
PredictLongRunningRequesti
endpoint (	BMâAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint:
	instances (2.google.protobuf.ValueBâAR	instances<

parameters (2.google.protobuf.ValueBâAR
parametersk
labels (2F.google.cloud.aiplatform.v1beta1.PredictLongRunningRequest.LabelsEntryBâA  ğ˜ìRlabels9
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
	http_body (2.google.api.HttpBodyBâARhttpBody"—
ExplainRequest\
endpoint (	B@âAúA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`€‹±ÍRendpoint:
	instances (2.google.protobuf.ValueBâAR	instances6

parameters (2.google.protobuf.ValueR
parameters”
explanation_spec_override (28.google.cloud.aiplatform.v1beta1.ExplanationSpecOverrideBòœâ¡v1beta1,v1,ui,internalRexplanationSpecOverrideÈ
$concurrent_explanation_spec_override (2V.google.cloud.aiplatform.v1beta1.ExplainRequest.ConcurrentExplanationSpecOverrideEntryBâAòœâ¡v1beta1,ui,internalR!concurrentExplanationSpecOverride@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelId
&ConcurrentExplanationSpecOverrideEntry
key (	RkeyN
value (28.google.cloud.aiplatform.v1beta1.ExplanationSpecOverrideRvalue:8"ƒ
ExplainResponseP
explanations (2,.google.cloud.aiplatform.v1beta1.ExplanationRexplanations¢
concurrent_explanations (2L.google.cloud.aiplatform.v1beta1.ExplainResponse.ConcurrentExplanationsEntryBòœâ¡v1beta1,ui,internalRconcurrentExplanations@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelId8
predictions (2.google.protobuf.ValueRpredictionsi
ConcurrentExplanationP
explanations (2,.google.cloud.aiplatform.v1beta1.ExplanationRexplanations‘
ConcurrentExplanationsEntry
key (	Rkey\
value (2F.google.cloud.aiplatform.v1beta1.ExplainResponse.ConcurrentExplanationRvalue:8"­
CountTokensRequestj
endpoint (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpointd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodel:
	instances (2.google.protobuf.ValueBâAR	instancesJ
contents (2(.google.cloud.aiplatform.v1beta1.ContentBâARcontentsb
system_instruction (2(.google.cloud.aiplatform.v1beta1.ContentBâAH RsystemInstructionˆA
tools (2%.google.cloud.aiplatform.v1beta1.ToolBâARtoolsi
generation_config (21.google.cloud.aiplatform.v1beta1.GenerationConfigBâAHRgenerationConfigˆB
_system_instructionB
_generation_config"ã
CountTokensResponse!
total_tokens (RtotalTokens:
total_billable_characters (RtotalBillableCharactersm
prompt_tokens_details (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountBâARpromptTokensDetails"Ê
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
speechResp:úÒä“VERTEX_PREDICTION_GRPC"¼
GenerateContentRequest}
endpoint (	BaâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`úÒä“GOOGLE_INTERNALÂöŒÜ"AUDITRendpointd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelJ
contents (2(.google.cloud.aiplatform.v1beta1.ContentBâARcontentsb
system_instruction (2(.google.cloud.aiplatform.v1beta1.ContentBâAH RsystemInstructionˆ
cached_content	 (	BgâAúA)
'aiplatform.googleapis.com/CachedContent  ğ˜Ì‚ÉÏ™`òœâ¡internal,v1beta1,v1ÂöŒÜ"AUDITRcachedContentA
tools (2%.google.cloud.aiplatform.v1beta1.ToolBâARtoolsY
tool_config (2+.google.cloud.aiplatform.v1beta1.ToolConfigBâA  ğ˜çR
toolConfigh
labels
 (2C.google.cloud.aiplatform.v1beta1.GenerateContentRequest.LabelsEntryBâA  ğ˜ìRlabels]
safety_settings (2..google.cloud.aiplatform.v1beta1.SafetySettingBâARsafetySettingse
model_armor_config (21.google.cloud.aiplatform.v1beta1.ModelArmorConfigBâARmodelArmorConfigd
generation_config (21.google.cloud.aiplatform.v1beta1.GenerationConfigBâARgenerationConfigK
service_account (	B"âA  ğ˜ÌúÒä“GOOGLE_INTERNALRserviceAccount@

session_id (	B!âA  ğ˜üúÒä“LLM_SESSION_IDR	sessionIdx
uniserve_endpoint_override (	B:âA  ğ˜ÌúÒä“)'VERTEX_UNISERVE_ENDPOINT_OVERRIDE_USERSRuniserveEndpointOverrideƒ
1global_endpoint_target_location_hint_testing_only (	BâAúÒä“GOOGLE_INTERNALR+globalEndpointTargetLocationHintTestingOnlyx
service_tier (2,.google.cloud.aiplatform.v1beta1.ServiceTierB"âA  ğ˜çúÒä“GOOGLE_INTERNALHRserviceTierˆ•
implicit_cache_config (23.google.cloud.aiplatform.v1beta1.ContextCacheConfigB'âAúÒä“DURABLE_CACHE_TRUSTED_USERSHRimplicitCacheConfigˆ]

admit_time (2.google.protobuf.TimestampB"âA  ğ˜çúÒä“GOOGLE_INTERNALR	admitTimeM
	fail_fast (B0âA  ğ˜çúÒä“GOOGLE_INTERNAL,FAIL_FAST_EAPRfailFast:
continuation_token (BâA  ğ˜ìRcontinuationTokeno
output_config (2D.google.cloud.aiplatform.v1beta1.GenerateContentRequest.OutputConfigBâARoutputConfigG
OutputConfig7
output_uri_prefix (	BâA  ğ˜ìRoutputUriPrefix9
LabelsEntry
key (	Rkey
value (	Rvalue:8:ÀöÄğ	B
_system_instructionB
_service_tierB
_implicit_cache_config"›
GenerateContentResponseP

candidates (2*.google.cloud.aiplatform.v1beta1.CandidateBâAR
candidates0
model_version (	BâA  ğ˜çRmodelVersionH
create_time (2.google.protobuf.TimestampBâA  ğ˜´R
createTime,
response_id (	BâA  ğ˜çR
responseId}
prompt_feedback (2G.google.cloud.aiplatform.v1beta1.GenerateContentResponse.PromptFeedbackBâA  ğ˜çRpromptFeedbackm
usage_metadata (2F.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadataRusageMetadata[
	send_time
 (2.google.protobuf.TimestampB"âA  ğ˜´úÒä“GOOGLE_INTERNALRsendTime¹
internal_debug_info (2J.google.cloud.aiplatform.v1beta1.GenerateContentResponse.InternalDebugInfoB=âAúÒä“MODEL_OPTIMIZER_TESTERòœâ¡internal,v1beta1,uiRinternalDebugInfo 
model_lifecycle_status (2M.google.cloud.aiplatform.v1beta1.GenerateContentResponse.ModelLifecycleStatusBâAúÒä“MODEL_LIFECYCLERmodelLifecycleStatus†
logprobs_result (2/.google.cloud.aiplatform.v1beta1.LogprobsResultB,  ğ˜ìúÒä“PROMPT_LOGPROBS_TRUSTED_USERSRlogprobsResultÜ
PromptFeedback…
block_reason (2U.google.cloud.aiplatform.v1beta1.GenerateContentResponse.PromptFeedback.BlockedReasonBâA  ğ˜çRblockReasona
safety_ratings (2-.google.cloud.aiplatform.v1beta1.SafetyRatingBâA  ğ˜çRsafetyRatings=
block_reason_message (	BâA  ğ˜çRblockReasonMessage"Ÿ
BlockedReason
BLOCKED_REASON_UNSPECIFIED 

SAFETY	
OTHER
	BLOCKLIST
PROHIBITED_CONTENT
MODEL_ARMOR
IMAGE_SAFETY
	JAILBREAK÷
UsageMetadata,
prompt_token_count (RpromptTokenCount4
candidates_token_count (RcandidatesTokenCount*
total_token_count (RtotalTokenCountB
tool_use_prompt_token_count (BâARtoolUsePromptTokenCount6
thoughts_token_count (BâARthoughtsTokenCountA
cached_content_token_count (BâARcachedContentTokenCountm
prompt_tokens_details	 (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountBâARpromptTokensDetailsk
cache_tokens_details
 (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountBâARcacheTokensDetailsu
candidates_tokens_details (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountBâARcandidatesTokensDetails}
tool_use_prompt_tokens_details (23.google.cloud.aiplatform.v1beta1.ModalityTokenCountBâARtoolUsePromptTokensDetails{
traffic_type (2R.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadata.TrafficTypeBâARtrafficType¨
billable_prompt_usage (2U.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadata.BillablleUsageBâAúÒä“GOOGLE_INTERNALRbillablePromptUsage·
billable_cached_content_usage (2U.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadata.BillablleUsageBâAúÒä“GOOGLE_INTERNALRbillableCachedContentUsageÀ
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
routed_model_name (	RroutedModelName:úÒä“MODEL_OPTIMIZER_TESTER–
ModelLifecycleStatus•
model_lifecycle_stage (2a.google.cloud.aiplatform.v1beta1.GenerateContentResponse.ModelLifecycleStatus.ModelLifecycleStageRmodelLifecycleStage'
retirement_date (	RretirementDate:
recommended_upgrade_model (	RrecommendedUpgradeModel"€
ModelLifecycleStage%
!MODEL_LIFECYCLE_STAGE_UNSPECIFIED 
EXPERIMENTAL
PREVIEW

STABLE

LEGACY
RETIRED"Í
CompleteCodeRequestd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodelS
prompts (23.google.cloud.aiplatform.v1beta1.CompleteCodePromptBâARpromptsd
completion_config (21.google.cloud.aiplatform.v1beta1.CompletionConfigBâARcompletionConfig:úÒä“UNIMPLEMENTED"Ö
CompleteCodeResponse]

candidates (27.google.cloud.aiplatform.v1beta1.CompleteCodePredictionBâAR
candidates@
deployed_model_id (	B  ğ˜ÌÂöŒÜ"AUDITRdeployedModelIdE
model (	B/âAúA!
aiplatform.googleapis.com/Model  ğ˜ÌRmodel5
model_version_id (	BâA  ğ˜ÌRmodelVersionId9
model_display_name (	BâA  ğ˜ìRmodelDisplayNameZ
metadata (2>.google.cloud.aiplatform.v1beta1.CompleteCodeResponse.MetadataRmetadatağ
Metadataz
output_token_count (2L.google.cloud.aiplatform.v1beta1.CompleteCodeResponse.Metadata.TokenMetadataRoutputTokenCountx
input_token_count (2L.google.cloud.aiplatform.v1beta1.CompleteCodeResponse.Metadata.TokenMetadataRinputTokenCountn
TokenMetadata:
total_billable_characters (RtotalBillableCharacters!
total_tokens (RtotalTokens:úÒä“UNIMPLEMENTED"û
ChatCompletionsRequestj
endpoint (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRendpoint>
	http_body (2.google.api.HttpBodyBâA  ğ˜ìRhttpBodyé	
	ExtraBody[
vertex_rag_store (2/.google.cloud.aiplatform.v1beta1.VertexRagStoreH RvertexRagStore‰
model_safety_settings (2U.google.cloud.aiplatform.v1beta1.ChatCompletionsRequest.ExtraBody.ModelSafetySettingsRmodelSafetySettings]
safety_settings (2..google.cloud.aiplatform.v1beta1.SafetySettingBâARsafetySettings,
thought_tag_marker (	RthoughtTagMarker%
cached_content (	RcachedContenti
thinking_config (2@.google.cloud.aiplatform.v1beta1.GenerationConfig.ThinkingConfigRthinkingConfig

session_id (	R	sessionId;
tools (2%.google.cloud.aiplatform.v1beta1.ToolRtoolsl
media_resolution	 (2A.google.cloud.aiplatform.v1beta1.GenerationConfig.MediaResolutionRmediaResolutionC
stream_function_call_arguments
 (RstreamFunctionCallArgumentsm
implicit_cache_config (23.google.cloud.aiplatform.v1beta1.ContextCacheConfigBâARimplicitCacheConfig¨
ModelSafetySettingsœ
llama_guard_settings (2h.google.cloud.aiplatform.v1beta1.ChatCompletionsRequest.ExtraBody.ModelSafetySettings.LlamaGuardSettingsH RllamaGuardSettings
enabled (BâARenabled-
LlamaGuardSettings:úÒä“GOOGLE_INTERNAL:úÒä“GOOGLE_INTERNALB

settings:úÒä“GOOGLE_INTERNALB
retrieval_sourceç
	ExtraPartW
video_metadata (2..google.cloud.aiplatform.v1beta1.VideoMetadataH RvideoMetadata
thought (HRthoughtˆ+
thought_signature (RthoughtSignature:úÒä“GOOGLE_INTERNALB

metadataB

_thoughtJ«
 PerChoiceResponseExtraPropertiesn
grounding_metadata (22.google.cloud.aiplatform.v1beta1.GroundingMetadataBâA  ğ˜çRgroundingMetadata:úÒä“GOOGLE_INTERNAL2
ResponseExtraProperties:úÒä“GOOGLE_INTERNALu
ExtraToolPartK
parts (25.google.cloud.aiplatform.v1beta1.FunctionResponsePartRparts:úÒä“GOOGLE_INTERNAL:ÀöÄğ	"£
PredictLongRunningResponsey
generate_video_response (26.google.cloud.aiplatform.v1beta1.GenerateVideoResponseB  ğ˜çH RgenerateVideoResponseB

response"
PredictLongRunningMetadata"ñ
FetchPredictOperationMetadatat

task_state (2H.google.cloud.aiplatform.v1beta1.FetchPredictOperationMetadata.TaskStateBâA  ğ˜çR	taskState"A
	TaskState
TASK_STATE_UNSPECIFIED 
STARTED
PENDING:úÒä“GOOGLE_INTERNAL"ê
GenerateVideoResponse/
generated_samples (	BRgeneratedSamplesE
rai_media_filtered_count (B  ğ˜çH RraiMediaFilteredCountˆD
rai_media_filtered_reasons (	B  ğ˜çRraiMediaFilteredReasons]
videos (2<.google.cloud.aiplatform.v1beta1.GenerateVideoResponse.VideoB  ğ˜ìRvideos–
Video"
gcs_uri (	B  ğ˜ìH RgcsUri;
bytes_base64_encoded (	B  ğ˜ìH RbytesBase64Encoded$
	mime_type (	B  ğ˜çRmimeTypeB
dataB
_rai_media_filtered_count"“
EmbedContentRequestu
model (	BZâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`’óèç
trueÂöŒÜ"AUDIT€‹±ÍH Rmodelˆ`
content (2(.google.cloud.aiplatform.v1beta1.ContentBâA  ğ˜ì’óèç
trueHRcontentˆ(
title (	BâA  ğ˜ìHRtitleˆw
	task_type (2F.google.cloud.aiplatform.v1beta1.EmbedContentRequest.EmbeddingTaskTypeBâA  ğ˜ì]HRtaskTypeˆ@
output_dimensionality (BâAHRoutputDimensionalityˆ0
auto_truncate (BâAHRautoTruncateˆ„
embed_content_config (2G.google.cloud.aiplatform.v1beta1.EmbedContentRequest.EmbedContentConfigBâAHRembedContentConfigˆv
service_account	 (	BHâAúA#
!iam.googleapis.com/ServiceAccount  ğ˜ÌúÒä“GOOGLE_INTERNALHRserviceAccountˆ‚
EmbedContentConfig&
title (	BâA  ğ˜ìH Rtitleˆu
	task_type (2F.google.cloud.aiplatform.v1beta1.EmbedContentRequest.EmbeddingTaskTypeBâA  ğ˜ì]HRtaskTypeˆ.
auto_truncate (BâAHRautoTruncateˆ>
output_dimensionality (BâAHRoutputDimensionalityˆ,
document_ocr (BâAHRdocumentOcrˆ?
audio_track_extraction (BâAHRaudioTrackExtractionˆB
_titleB

_task_typeB
_auto_truncateB
_output_dimensionalityB
_document_ocrB
_audio_track_extraction"ú
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
_service_accountJ"•
EmbedContentResponse]
	embedding (2?.google.cloud.aiplatform.v1beta1.EmbedContentResponse.EmbeddingR	embeddingU
usage_metadata (2..google.cloud.aiplatform.v1beta1.UsageMetadataRusageMetadata
	truncated (R	truncated#
	Embedding
values (RvaluesJ"ı
BatchEmbedContentsRequestd
model (	BNâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`ÂöŒÜ"AUDIT€‹±ÍRmodel]
requests (24.google.cloud.aiplatform.v1beta1.EmbedContentRequestBâA  ğ˜ìRrequests:òœâ¡internal,v1beta1,v1"·
BatchEmbedContentsResponsen

embeddings (2E.google.cloud.aiplatform.v1beta1.BatchEmbedContentsResponse.EmbeddingB  ğ˜çR
embeddings^
usage_metadata (2..google.cloud.aiplatform.v1beta1.UsageMetadataB  ğ˜çRusageMetadata,
	Embedding
values (B  ğ˜çRvalues:òœâ¡internal,v1beta1,v1"í
%AsyncGenerateContentOperationMetadatad
generic_metadata (29.google.cloud.aiplatform.v1beta1.GenericOperationMetadataRgenericMetadata*

session_id (	BâA  ğ˜üR	sessionIds
usage_metadata (2F.google.cloud.aiplatform.v1beta1.GenerateContentResponse.UsageMetadataBâARusageMetadata#

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
"aiplatform.googleapis.com/Response  ğ˜Ì€‹±ÍRname:òœâ¡internal,v1beta1,ui,v1"¹
DetectContentRequestu
model (	BZâA
úA$
"aiplatform.googleapis.com/Endpoint  ğ˜Ì‚ÉÏ™`’óèç
trueÂöŒÜ"AUDIT€‹±ÍH Rmodelˆ`
content (2(.google.cloud.aiplatform.v1beta1.ContentBâA  ğ˜ì’óèç
trueHRcontentˆ:2úÒä“GOOGLE_INTERNALòœâ¡internal,v1beta1,v1B
_modelB

_content"ø
DetectContentResponse‘
ai_generated_likelihood (2L.google.cloud.aiplatform.v1beta1.DetectContentResponse.AiGeneratedLikelihoodBâA  ğ˜çRaiGeneratedLikelihoodb
c2pa (2A.google.cloud.aiplatform.v1beta1.DetectContentResponse.C2paResultBâA  ğ˜ìRc2pa,
response_id (	BâA  ğ˜çR
responseId=
model_version (	BâA  ğ˜çÂöŒÜ"AUDITRmodelVersionÇ

C2paResultp
outcome (2I.google.cloud.aiplatform.v1beta1.DetectContentResponse.C2paResult.OutcomeBâA  ğ˜çRoutcomes
manifest (2J.google.cloud.aiplatform.v1beta1.DetectContentResponse.C2paResult.ManifestBâA  ğ˜ìRmanifestÙ
Manifest‹
creation_types (2W.google.cloud.aiplatform.v1beta1.DetectContentResponse.C2paResult.Manifest.CreationTypeBâA  ğ˜çRcreationTypesx
actions (2Q.google.cloud.aiplatform.v1beta1.DetectContentResponse.C2paResult.Manifest.ActionBâA  ğ˜çRactions<
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
	VERY_HIGH:2úÒä“GOOGLE_INTERNALòœâ¡internal,v1beta1,v12Ûf
PredictionService
Predict/.google.cloud.aiplatform.v1beta1.PredictRequest0.google.cloud.aiplatform.v1beta1.PredictResponse"ŸÚAendpoint,instances,parameters‚Óä“’:*ZM:*"H/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:predict">/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:predictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phó

RawPredict2.google.cloud.aiplatform.v1beta1.RawPredictRequest.google.api.HttpBody"šÚAendpoint,http_body‚Óä“˜:*ZP:*"K/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:rawPredict"A/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:rawPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph
StreamRawPredict8.google.cloud.aiplatform.v1beta1.StreamRawPredictRequest.google.api.HttpBody"¦ÚAendpoint,http_body‚Óä“¤:*ZV:*"Q/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:streamRawPredict"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:streamRawPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0¶
DirectPredict5.google.cloud.aiplatform.v1beta1.DirectPredictRequest6.google.cloud.aiplatform.v1beta1.DirectPredictResponse"µ‚Óä“I:*"D/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:directPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÂ
DirectRawPredict8.google.cloud.aiplatform.v1beta1.DirectRawPredictRequest9.google.cloud.aiplatform.v1beta1.DirectRawPredictResponse"¸‚Óä“L:*"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:directRawPredictª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phı
StreamDirectPredict;.google.cloud.aiplatform.v1beta1.StreamDirectPredictRequest<.google.cloud.aiplatform.v1beta1.StreamDirectPredictResponse"æª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0†
StreamDirectRawPredict>.google.cloud.aiplatform.v1beta1.StreamDirectRawPredictRequest?.google.cloud.aiplatform.v1beta1.StreamDirectRawPredictResponse"æª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0÷
StreamingPredict8.google.cloud.aiplatform.v1beta1.StreamingPredictRequest9.google.cloud.aiplatform.v1beta1.StreamingPredictResponse"éª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phˆ(0Ä
ServerStreamingPredict8.google.cloud.aiplatform.v1beta1.StreamingPredictRequest9.google.cloud.aiplatform.v1beta1.StreamingPredictResponse"²‚Óä“°:*Z\:*"W/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:serverStreamingPredict"M/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:serverStreamingPredictòœâ¡ui,v1,v1beta1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ı
StreamingRawPredict;.google.cloud.aiplatform.v1beta1.StreamingRawPredictRequest<.google.cloud.aiplatform.v1beta1.StreamingRawPredictResponse"æª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph(0Â
PredictLongRunning:.google.cloud.aiplatform.v1beta1.PredictLongRunningRequest.google.longrunning.Operation"ĞÚAendpoint,instances,parameters‚Óä“¨:*ZX:*"S/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:predictLongRunning"I/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:predictLongRunningòœâ¡internal,v1beta1,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@PhÈ
FetchPredictOperation=.google.cloud.aiplatform.v1beta1.FetchPredictOperationRequest.google.longrunning.Operation"ĞÚAendpoint,operation_name‚Óä“®:*Z[:*"V/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:fetchPredictOperation"L/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:fetchPredictOperationòœâ¡internal,v1beta1,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phõ
Invoke..google.cloud.aiplatform.v1beta1.InvokeRequest.google.api.HttpBody"¢ÚAendpoint‚Óä“Œ:*Zh:*"c/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/deployedModels/{deployed_model_id}/invoke/**ZT:	http_body"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/openapi}/embeddingsZU:	http_body"H/v1beta1/{endpoint=projects/*/locations/*/endpoints/openapi}/completionsZS:	http_body"F/v1beta1/{endpoint=projects/*/locations/*/endpoints/openapi}/responsesZQ:	http_body"D/v1beta1/{endpoint=projects/*/locations/*/publishers/*}/v1/responsesZY:	http_body"L/v1beta1/{endpoint=projects/*/locations/*/publishers/*}/v1/responses/compactZZ:	http_body"M/v1beta1/{endpoint=projects/*/locations/*/endpoints/google}/science/inferenceZO:*"J/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}/invoke/**"@/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invoke/**òœâ¡internal,v1beta1,ui,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ê
StreamPublisherInvoke=.google.cloud.aiplatform.v1beta1.StreamPublisherInvokeRequest.google.api.HttpBody"÷ÚAendpoint‚Óä“\r$HIDE_FROM_DISCOVERYD/v1beta1/{endpoint=projects/*/locations/*/publishers/*}/v1/responsesòœâ¡internal,v1,v1beta1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      ¬@Ph(0å
	InvokeApi1.google.cloud.aiplatform.v1beta1.InvokeApiRequest.google.api.HttpBody"ŒÚAendpointúÒä“ GOOGLE_INTERNAL,TRUSTED_TESTER‚Óä“Ö:*ZEC/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**ZE*C/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**"C/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/invokeApi/**òœâ¡internal,v1beta1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0ã
GetResponse3.google.cloud.aiplatform.v1beta1.GetResponseRequest.google.api.HttpBody"ˆÚAname‚Óä“†ZDB/v1beta1/{name=projects/*/locations/*/publishers/*/v1/responses/*}>/v1beta1/{name=projects/*/locations/*/endpoints/*/responses/*}òœâ¡internal,v1beta1,ui,v1ª»ÉéË†
name'aiplatform.googleapis.com/responses.get+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Phú
DeleteResponse6.google.cloud.aiplatform.v1beta1.DeleteResponseRequest.google.api.HttpBody"™ÚAname‚Óä“†ZD*B/v1beta1/{name=projects/*/locations/*/publishers/*/v1/responses/*}*>/v1beta1/{name=projects/*/locations/*/endpoints/*/responses/*}òœâ¡internal,v1beta1,ui,v1ª»ÉéÜŠ
name*aiplatform.googleapis.com/responses.delete+RESOURCE,aiplatform.googleapis.com/Response"AUDIT,AUDIT_TARGET_RESOURCE0b
DATA_WRITE";CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION,WRITE_ACTION2AUTO_CLOUD_IAM@PhĞ
Explain/.google.cloud.aiplatform.v1beta1.ExplainRequest0.google.cloud.aiplatform.v1beta1.ExplainResponse"áÚA/endpoint,instances,parameters,deployed_model_id‚Óä“C:*">/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:explainª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.explain+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph§
CountTokens3.google.cloud.aiplatform.v1beta1.CountTokensRequest4.google.cloud.aiplatform.v1beta1.CountTokensResponse"¬ÚAendpoint,instances‚Óä“š:*ZQ:*"L/v1beta1/{endpoint=projects/*/locations/*/publishers/*/models/*}:countTokens"B/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}:countTokensòœâ¡internal,v1beta1ª»ÉéÓ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ".CLOUD_AUDIT_REQUEST_ONLY,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph
GenerateContent7.google.cloud.aiplatform.v1beta1.GenerateContentRequest8.google.cloud.aiplatform.v1beta1.GenerateContentResponse"—ÚAmodel,contents‚Óä“œ:*ZR:*"M/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:generateContent"C/v1beta1/{model=projects/*/locations/*/endpoints/*}:generateContentª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph²
StreamGenerateContent7.google.cloud.aiplatform.v1beta1.GenerateContentRequest8.google.cloud.aiplatform.v1beta1.GenerateContentResponse"£ÚAmodel,contents‚Óä“¨:*ZX:*"S/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:streamGenerateContent"I/v1beta1/{model=projects/*/locations/*/endpoints/*}:streamGenerateContentª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0Ø
ChatCompletions7.google.cloud.aiplatform.v1beta1.ChatCompletionsRequest.google.api.HttpBody"óÚAendpoint,http_body‚Óä“T:	http_body"G/v1beta1/{endpoint=projects/*/locations/*/endpoints/*}/chat/completionsòœâ¡ui,internal,v1beta1,v1ª»ÉéÛ
endpoint+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@Ph0…
DetectContent5.google.cloud.aiplatform.v1beta1.DetectContentRequest6.google.cloud.aiplatform.v1beta1.DetectContentResponse"„ÚAmodel,contentúÒä“GOOGLE_INTERNAL‚Óä“P:*"K/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:detectContentòœâ¡internal,v1beta1,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      N@Phê
EmbedContent4.google.cloud.aiplatform.v1beta1.EmbedContentRequest5.google.cloud.aiplatform.v1beta1.EmbedContentResponse"ìÚAmodel,content‚Óä“O:*"J/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:embedContentòœâ¡v1beta1,internal,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A      N@Phƒ
BatchEmbedContents:.google.cloud.aiplatform.v1beta1.BatchEmbedContentsRequest;.google.cloud.aiplatform.v1beta1.BatchEmbedContentsResponse"óÚAmodel,requests‚Óä“U:*"P/v1beta1/{model=projects/*/locations/*/publishers/*/models/*}:batchEmbedContentsòœâ¡internal,v1beta1,v1ª»ÉéØ‹
model+aiplatform.googleapis.com/endpoints.predict+RESOURCE,aiplatform.googleapis.com/Endpoint"AUDIT,AUDIT_TARGET_RESOURCE0b	DATA_READ"6CLOUD_AUDIT_REQUEST_AND_RESPONSE,GIN_AUDIT,READ_ACTION2AUTO_CLOUD_IAM@A     À‚@PhBÎŠÕÛÒ
all
#com.google.cloud.aiplatform.v1beta1BPredictionServiceProtoPªGoogle.Cloud.AIPlatform.V1Beta1ÊGoogle\Cloud\AIPlatform\V1beta1ê"Google::Cloud::AIPlatform::V1beta1òOPEN_TO_OPAQUE_HYBRID bproto3
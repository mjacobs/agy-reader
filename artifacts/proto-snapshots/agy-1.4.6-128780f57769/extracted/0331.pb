
;third_party/jetski/seat_management_pb/seat_management.protoexa.seat_management_pbgoogle/protobuf/timestamp.proto9third_party/jetski/codeium_common_pb/codeium_common.proto"¿

User
api_key (	RapiKey
name (	Rname
email (	Remail;
signup_time (2.google.protobuf.TimestampR
signupTimeD
last_update_time (2.google.protobuf.TimestampRlastUpdateTime
id (	Rid
team_id (	RteamIdF
team_status (2%.exa.codeium_common_pb.UserTeamStatusR
teamStatus
username	 (	Rusername.
preferred_time_zone
 (	RpreferredTimeZone4
public_profile_enabled (RpublicProfileEnabled
bio (	Rbio
pro (Rpro.
profile_picture_uri (	RprofilePictureUri'
disable_codeium (RdisableCodeiumA
profile_picture_presigned_url (	RprofilePicturePresignedUrl-
occupation_company (	RoccupationCompany'
occupation_role (	RoccupationRole5
years_of_experience_min (RyearsOfExperienceMin5
years_of_experience_max (RyearsOfExperienceMax

newsletter (R
newsletter-
disabled_telemetry (RdisabledTelemetry%
inbound_source (	RinboundSource!
signup_stage (	RsignupStage

used_trial (R	usedTrialW
first_antigravity_use_time (2.google.protobuf.TimestampRfirstAntigravityUseTime^
antigravity_pro_trial_end_time (2.google.protobuf.TimestampRantigravityProTrialEndTime2
used_prompt_credits (BRusedPromptCredits.
used_flow_credits (BRusedFlowCredits#
referral_code (	RreferralCode"û

Team
id (	Rid
name (	Rname;
signup_time (2.google.protobuf.TimestampR
signupTime
	invite_id (	RinviteId

used_trial (R	usedTrial4
stripe_subscription_id (	RstripeSubscriptionId/
subscription_active (RsubscriptionActive,
stripe_customer_id (	RstripeCustomerId[
current_billing_period_start	 (2.google.protobuf.TimestampRcurrentBillingPeriodStartW
current_billing_period_end (2.google.protobuf.TimestampRcurrentBillingPeriodEndF
 num_seats_current_billing_period
 (RnumSeatsCurrentBillingPeriod/
attribution_enabled (RattributionEnabled&
sso_provider_id (	RssoProviderId%
offers_enabled (RoffersEnabled?

teams_tier (2 .exa.codeium_common_pb.TeamsTierR	teamsTier*
flex_credit_quota (RflexCreditQuota*
used_flow_credits (RusedFlowCredits.
used_prompt_credits (RusedPromptCredits*
num_cascade_seats (RnumCascadeSeatsU
cascade_usage_month_start (2.google.protobuf.TimestampRcascadeUsageMonthStartQ
cascade_usage_month_end (2.google.protobuf.TimestampRcascadeUsageMonthEndS
cascade_seat_type (2'.exa.seat_management_pb.CascadeSeatTypeRcascadeSeatType$
top_up_enabled (RtopUpEnabled1
monthly_top_up_amount (RmonthlyTopUpAmount 
top_up_spent (R
topUpSpent(
top_up_increment (RtopUpIncrement*
used_flex_credits (RusedFlexCredits"…
PreapprovedUser
id (	Rid
name (	Rname
email (	Remail
team_id (	RteamId)
issued_by_api_key (	RissuedByApiKey9

created_at (2.google.protobuf.TimestampR	createdAt9

updated_at (2.google.protobuf.TimestampR	updatedAt"¿
UserSettings
exists (Rexists

newsletter (R
newsletter-
disabled_telemetry (RdisabledTelemetry%
inbound_source (	RinboundSource!
signup_stage (	RsignupStage"o
UserRole
api_key (	RapiKey
roles (	Rroles
role_id (	RroleId
	role_name (	RroleName"S
GetUserStatusRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"™
GetUserStatusResponseB
user_status (2!.exa.codeium_common_pb.UserStatusR
userStatus<
	plan_info (2.exa.codeium_common_pb.PlanInfoRplanInfo"¨
GetCurrentUserRequest

auth_token (	R	authToken?
generate_profile_picture_url (RgenerateProfilePictureUrl-
create_if_not_exist (RcreateIfNotExist1
include_subscription (RincludeSubscription
	sso_token (	BRssoToken,
saml_provider_id (	BRsamlProviderId"ë
ReferralInfo#
referrer_name (	RreferrerName%
referrer_email (	RreferrerEmailP
referral_accepted_time (2.google.protobuf.TimestampRreferralAcceptedTime=
granted_time (2.google.protobuf.TimestampRgrantedTime"®
GetCurrentUserResponse0
user (2.exa.seat_management_pb.UserRuser
roles (	Rroles0
team (2.exa.seat_management_pb.TeamRteamH
subscription (2$.exa.seat_management_pb.UserSettingsRsubscriptionC
permissions (2!.exa.codeium_common_pb.PermissionRpermissions4
role (2 .exa.seat_management_pb.UserRoleRrole<
	plan_info (2.exa.codeium_common_pb.PlanInfoRplanInfoS
user_team_details	 (2'.exa.seat_management_pb.UserTeamDetailsRuserTeamDetails@
referral
 (2$.exa.seat_management_pb.ReferralInfoRreferrale
team_features (2@.exa.seat_management_pb.GetCurrentUserResponse.TeamFeaturesEntryRteamFeatures
	group_ids (	RgroupIdsm
TeamFeaturesEntry
key (RkeyB
value (2,.exa.codeium_common_pb.TeamsFeaturesMetadataRvalue:8JRlicense"K
GetUsersRequest

auth_token (	R	authToken
group_id (	RgroupId"º
GetUsersResponse2
users (2.exa.seat_management_pb.UserRusers?

user_roles (2 .exa.seat_management_pb.UserRoleR	userRolesS
user_team_details (2'.exa.seat_management_pb.UserTeamDetailsRuserTeamDetails\
user_cascade_details (2*.exa.seat_management_pb.UserCascadeDetailsRuserCascadeDetails"P
GetRolesForUserRequest

auth_token (	R	authToken
api_key (	RapiKey"/
GetRolesForUserResponse
roles (	Rroles"œ
AddUserRoleRequest

auth_token (	R	authToken
api_key (	RapiKey
role (	Rrole
group_id (	RgroupId
service_key (	R
serviceKey"
AddUserRoleResponse"~
RemoveUserRoleRequest

auth_token (	R	authToken
api_key (	RapiKey
role (	Rrole
group_id (	RgroupId"
RemoveUserRoleResponse"C
RegisterUserRequest,
firebase_id_token (	Rfirebase_id_token"l
RegisterUserResponse
api_key (	Rapi_key
name (	Rname&
api_server_url (	Rapi_server_url"0
MigrateApiKeyRequest
api_key (	Rapi_key"=
MigrateApiKeyResponse$
session_token (	Rsession_token"O
UpdateNameRequest

auth_token (	R	authToken
	full_name (	RfullName"
UpdateNameResponse"K
*GetProfilePicturePresignedUploadUrlRequest

auth_token (	R	authToken"?
+GetProfilePicturePresignedUploadUrlResponse
url (	Rurl"D
#ProfilePictureUploadCompleteRequest

auth_token (	R	authToken"&
$ProfilePictureUploadCompleteResponse"0
GetProfileDataRequest
api_key (	RapiKey"H
GetProfileDataResponse.
profile_picture_url (	RprofilePictureUrl"<
DeleteProfilePictureRequest

auth_token (	R	authToken"
DeleteProfilePictureResponse"b
UpdateInboundSourceRequest

auth_token (	R	authToken%
inbound_source (	RinboundSource"
UpdateInboundSourceResponse"K
DeleteUserRequest

auth_token (	R	authToken
api_key (	RapiKey"
DeleteUserResponse"P
RequestTeamAccessRequest
api_key (	RapiKey
	invite_id (	RinviteId"
RequestTeamAccessResponse"5
GetTeamMetadataRequest
	invite_id (	RinviteId"K
GetTeamMetadataResponse0
team (2.exa.seat_management_pb.TeamRteam"S
RemoveUserFromTeamRequest

auth_token (	R	authToken
api_key (	RapiKey"
RemoveUserFromTeamResponse";
RefreshTeamInviteIdRequest

auth_token (	R	authToken"O
RefreshTeamInviteIdResponse0
team (2.exa.seat_management_pb.TeamRteam"â
GrantPreapprovalRequest

auth_token (	R	authTokeng
preapprovals (2C.exa.seat_management_pb.GrantPreapprovalRequest.PreapprovalUserItemRpreapprovals?
PreapprovalUserItem
name (	Rname
email (	Remail"g
GrantPreapprovalResponseK
preapprovals (2'.exa.seat_management_pb.PreapprovedUserRpreapprovals"7
GetPreapprovalsRequest

auth_token (	R	authToken"f
GetPreapprovalsResponseK
preapprovals (2'.exa.seat_management_pb.PreapprovedUserRpreapprovals"@
GetPreapprovalMetadataRequest
approval_id (	R
approvalId"g
GetPreapprovalMetadataResponse
name (	Rname
email (	Remail
	team_name (	RteamName"Z
RejectPreapprovalRequest

auth_token (	R	authToken
approval_id (	R
approvalId"
RejectPreapprovalResponse"=
GetPreapprovalForUserRequest

auth_token (	R	authToken"¦
GetPreapprovalForUserResponseI
preapproval (2'.exa.seat_management_pb.PreapprovedUserRpreapproval

admin_name (	R	adminName
	team_name (	RteamName"Z
RevokePreapprovalRequest

auth_token (	R	authToken
approval_id (	R
approvalId"
RevokePreapprovalResponse"Õ
BillingUpdate4
amount_due_immediately (RamountDueImmediately$
price_per_seat (RpricePerSeat
	num_seats (RnumSeatsF
sub_interval (2#.exa.seat_management_pb.SubIntervalRsubInterval.
amount_per_interval (RamountPerInterval?
billing_start (2.google.protobuf.TimestampRbillingStart;
billing_end (2.google.protobuf.TimestampR
billingEnd0
unused_plan_refunded	 (RunusedPlanRefunded#
has_sso_add_on
 (RhasSsoAddOn";
GetUserSubscriptionRequest

auth_token (	R	authToken"˜
GetUserSubscriptionResponse
exists (Rexists

newsletter (R
newsletter-
disabled_telemetry (RdisabledTelemetry%
inbound_source (	RinboundSource!
signup_stage (	RsignupStageH
subscription (2$.exa.seat_management_pb.UserSettingsRsubscription"W
IsUsernameAvailableRequest

auth_token (	R	authToken
username (	Rusername";
IsUsernameAvailableResponse
	available (R	available"R
UpdateUsernameRequest

auth_token (	R	authToken
username (	Rusername"
UpdateUsernameResponse"þ
UpdateOccupationRequest

auth_token (	R	authToken-
occupation_company (	RoccupationCompany'
occupation_role (	RoccupationRole5
years_of_experience_min (RyearsOfExperienceMin5
years_of_experience_max (RyearsOfExperienceMax"
UpdateOccupationResponse"¡
UpdateProfileRequest

auth_token (	R	authToken
new_name (	RnewName
	new_email (	RnewEmail
new_bio (	RnewBio
otp_code (	RotpCode"
UpdateProfileResponse"u
UpdateProfileVisibilityRequest

auth_token (	R	authToken4
public_profile_enabled (RpublicProfileEnabled"!
UpdateProfileVisibilityResponse"5
GetPublicProfileRequest
username (	Rusername"Œ
GetPublicProfileResponse
name (	Rname
username (	Rusername.
preferred_time_zone (	RpreferredTimeZone;
signup_time (2.google.protobuf.TimestampR
signupTime
bio (	RbioA
profile_picture_presigned_url (	RprofilePicturePresignedUrl"\
UpdateUserRolesRequest
secret (	Rsecret
email (	Remail
roles (	Rroles"
UpdateUserRolesResponse"ˆ
BulkUpdateUserRolesRequest
service_key (	R
serviceKey
emails (	Remails
role (	Rrole

group_name (	R	groupName"o
BulkUpdateUserRolesResponse+
successful_emails (	RsuccessfulEmails#
failed_emails (	RfailedEmails"6
GetSSOProviderRequest

auth_token (	R	authToken"p
GetSSOProviderResponseV
saml_auth_provider (2(.exa.seat_management_pb.SAMLAuthProviderRsamlAuthProvider"¨
SaveSSOProviderRequest

auth_token (	R	authTokenV
saml_auth_provider (2(.exa.seat_management_pb.SAMLAuthProviderRsamlAuthProvider
is_test (RisTest"
SaveSSOProviderResponse"¼
SAMLAuthProvider&
sso_provider_id (	RssoProviderId"
idp_entity_id (	RidpEntityId
sso_url (	RssoUrl)
x509_certificate (	Rx509Certificate
enabled (Renabled"3
UserSSOLoginRedirectRequest
email (	Remail"×
UserSSOLoginRedirectResponse'
should_redirect (RshouldRedirect&
sso_provider_id (	RssoProviderIdC
disallow_enterprise_user_login (RdisallowEnterpriseUserLogin!
redirect_url (	RredirectUrl"ê
EnterpriseLicenseConfigM
expiration_timestamp (2.google.protobuf.TimestampRexpirationTimestamp

seat_limit (R	seatLimit/
attribution_enabled (RattributionEnabled,
audit_logs_enabled (RauditLogsEnabledC
start_timestamp (2.google.protobuf.TimestampRstartTimestamp

mucs_total (R	mucsTotal!
mucs_enabled (RmucsEnabled2
license_length_months (RlicenseLengthMonthsG
 enable_individual_user_analytics	 (RenableIndividualUserAnalytics"2
DeleteTeamRequest

auth_token (	R	authToken"
DeleteTeamResponse"8
GetTeamsFeaturesRequest

auth_token (	R	authToken"\
GetTeamsFeaturesResponse@
features (2$.exa.codeium_common_pb.TeamsFeaturesRfeatures"R
GetTeamsFeaturesInternalRequest
secret (	Rsecret
team_id (	RteamId"d
 GetTeamsFeaturesInternalResponse@
features (2$.exa.codeium_common_pb.TeamsFeaturesRfeatures"í
"UpdateTeamsFeaturesInternalRequest
secret (	Rsecret
team_id (	RteamIdG
add_features (2$.exa.codeium_common_pb.TeamsFeaturesRaddFeaturesM
remove_features (2$.exa.codeium_common_pb.TeamsFeaturesRremoveFeatures"Œ
SetTeamsFeaturesRequest
secret (	Rsecret
team_id (	RteamId@
features (2$.exa.codeium_common_pb.TeamsFeaturesRfeatures"%
#UpdateTeamsFeaturesInternalResponse"
SetTeamsFeaturesResponse"—
!UpdateUserFeaturesInternalRequest
secret (	Rsecret
api_keys (	RapiKeys?
features (2#.exa.codeium_common_pb.UserFeaturesRfeatures"$
"UpdateUserFeaturesInternalResponse"Á
BulkEditUserApprovalsRequest

auth_token (	R	authToken
emails (	RemailsP
approval_type (2+.exa.seat_management_pb.UserTeamDetailsTypeRapprovalType
approve (Rapprove"o
UserTeamDetails
auth_uid (	RauthUid
team_id (	RteamId(
has_cascade_seat (RhasCascadeSeat"
BulkEditUserApprovalsResponse"ß
$BulkEditUserApprovalsInternalRequest
secret (	Rsecret
	team_name (	RteamName
emails (	RemailsP
approval_type (2+.exa.seat_management_pb.UserTeamDetailsTypeRapprovalType
approve (Rapprove"'
%BulkEditUserApprovalsInternalResponse"7
GetTeamSettingsRequest

auth_token (	R	authToken"§
GetTeamSettingsResponse;
cascade_web_search_enabled (RcascadeWebSearchEnabledO
%can_update_cascade_web_search_enabled (R canUpdateCascadeWebSearchEnabled"‚
$UpdateCascadeWebSearchEnabledRequest

auth_token (	R	authToken;
cascade_web_search_enabled (RcascadeWebSearchEnabled"'
%UpdateCascadeWebSearchEnabledResponse"t
GetGroupsRequest

auth_token (	R	authTokenA

permission (2!.exa.codeium_common_pb.PermissionR
permission"+
Group
id (	Rid
name (	Rname"J
GetGroupsResponse5
groups (2.exa.seat_management_pb.GroupRgroups"~
CreateTeamApiSecretRequest

auth_token (	R	authToken
name (	Rname
role (	Rrole
group_id (	RgroupId"5
CreateTeamApiSecretResponse
secret (	Rsecret"¤
UpdateTeamApiSecretRequest

auth_token (	R	authToken$
service_key_id (	RserviceKeyId
name (	Rname
role (	Rrole
group_id (	RgroupId"
UpdateTeamApiSecretResponse"<
GetAllTeamApiSecretsRequest

auth_token (	R	authToken"¢
TeamApiSecret
id (	Rid
name (	Rname9

created_at (2.google.protobuf.TimestampR	createdAt
role_id (	RroleId
group_id (	RgroupId"_
GetAllTeamApiSecretsResponse?
secrets (2%.exa.seat_management_pb.TeamApiSecretRsecrets"X
DeleteTeamApiSecretRequest

auth_token (	R	authToken
	secret_id (	RsecretId"
DeleteTeamApiSecretResponse"6
GetUserNotificationsRequest
api_key (	RapiKey"¼
GetUserNotificationsResponseˆ
team_preapproval_metadata (2L.exa.seat_management_pb.GetUserNotificationsResponse.TeamPreapprovalMetadataRteamPreapprovalMetadata
TeamPreapprovalMetadata

admin_name (	R	adminName
	team_name (	RteamName9

created_at (2.google.protobuf.TimestampR	createdAt"Ð
ConnectGithubAccountRequest

auth_token (	R	authToken0
github_refresh_token (	RgithubRefreshToken`
github_refresh_token_expires_at (2.google.protobuf.TimestampRgithubRefreshTokenExpiresAt"®
ConnectGithubAccountResponse.
github_access_token (	RgithubAccessToken^
github_access_token_expires_at (2.google.protobuf.TimestampRgithubAccessTokenExpiresAt"?
DisconnectGithubAccountRequest

auth_token (	R	authToken"!
DisconnectGithubAccountResponse">
GetGitHubAccountStatusRequest

auth_token (	R	authToken"•
GetGitHubAccountStatusResponse
	connected (R	connected
username (	Rusername9

expires_at (2.google.protobuf.TimestampR	expiresAt"<
GetGitHubAccessTokenRequest

auth_token (	R	authToken"|
GetGitHubAccessTokenResponse!
access_token (	RaccessToken9

expires_at (2.google.protobuf.TimestampR	expiresAt"Ò
ConnectNetlifyAccountRequest

auth_token (	R	authToken0
netlify_access_token (	RnetlifyAccessToken9

expires_at (2.google.protobuf.TimestampR	expiresAt&
netlify_user_id (	RnetlifyUserId"
ConnectNetlifyAccountResponse"@
DisconnectNetlifyAccountRequest

auth_token (	R	authToken""
 DisconnectNetlifyAccountResponse"?
GetNetlifyAccountStatusRequest

auth_token (	R	authToken"¢
GetNetlifyAccountStatusResponse
	connected (R	connected&
netlify_user_id (	RnetlifyUserId9

expires_at (2.google.protobuf.TimestampR	expiresAt"‹
CreateRoleRequest

auth_token (	R	authToken
name (	RnameC
permissions (2!.exa.codeium_common_pb.PermissionRpermissions"
CreateRoleResponse"K
DeleteRoleRequest

auth_token (	R	authToken
role_id (	RroleId"
DeleteRoleResponse"
UpdateRoleRequest

auth_token (	R	authToken
role_id (	RroleIdC
permissions (2!.exa.codeium_common_pb.PermissionRpermissions"
UpdateRoleResponse"0
GetRolesRequest

auth_token (	R	authToken"’
Role
role_id (	RroleId
name (	RnameC
permissions (2!.exa.codeium_common_pb.PermissionRpermissions
mutable (Rmutable"F
GetRolesResponse2
roles (2.exa.seat_management_pb.RoleRroles"
GetMucsInfoRequest"¨
MonthlyMucsMessage%
billing_months (RbillingMonths#
billing_years (RbillingYears!
mucs_charged (RmucsCharged#
mucs_deducted (RmucsDeducted"í
GetMucsInfoResponse
	mucs_left (RmucsLeft
	mucs_used (RmucsUsed&
is_mucs_enabled (RisMucsEnabled9

start_date (2.google.protobuf.TimestampR	startDate8
current_registered_users (RcurrentRegisteredUsers4
current_high_watermark (RcurrentHighWatermarkI

mucs_table (2*.exa.seat_management_pb.MonthlyMucsMessageR	mucsTable"Á
GetCascadeAnalyticsRequest

auth_token (	R	authTokenC
start_timestamp (2.google.protobuf.TimestampRstartTimestamp?
end_timestamp (2.google.protobuf.TimestampRendTimestamp"š
GetCascadeAnalyticsResponse.
used_prompt_credits (RusedPromptCredits*
used_flow_credits (RusedFlowCredits:
estimated_monthly_prompts (RestimatedMonthlyPrompts6
estimated_monthly_flows (RestimatedMonthlyFlows+
purchased_credits (RpurchasedCredits"5
GetPlanStatusRequest

auth_token (	R	authToken"[
GetPlanStatusResponseB
plan_status (2!.exa.codeium_common_pb.PlanStatusR
planStatus"ª
Wrapped2024
api_key (	RapiKey
email (	Remail
name (	Rname.
total_minutes_coded (RtotalMinutesCoded+
coding_percentile (RcodingPercentileH
biggest_coding_day (2.google.protobuf.TimestampRbiggestCodingDay;
minutes_coding_biggest_day (RminutesCodingBiggestDay5
autocompletes_accepted (RautocompletesAccepted2
cascade_messages_sent	 (RcascadeMessagesSent2
cascade_lines_written
 (RcascadeLinesWritten%
longest_streak (RlongestStreak.
most_productive_day (RmostProductiveDay"6
GetWrapped2024Request

auth_token (	R	authToken"`
GetWrapped2024ResponseF
wrapped_data (2#.exa.seat_management_pb.Wrapped2024RwrappedData"ç
FlexCreditChronicleEntry
team_id (	RteamId9

grant_date (2.google.protobuf.TimestampR	grantDate
num_credits (R
numCreditsC
type (2/.exa.seat_management_pb.FlexCreditChronicleTypeRtype
referral_id (R
referralId

invoice_id (	R	invoiceIdD
referrer (2&.exa.seat_management_pb.ReferrerReasonH Rreferrer;
avery (2#.exa.seat_management_pb.AveryReasonH RaveryD
purchase	 (2&.exa.seat_management_pb.PurchaseReasonH RpurchaseB
reason"P
ReferrerReason

user_email (	R	userEmail
avery_email (	R
averyEmail"S
AveryReason

user_email (	R	userEmail%
referrer_email (	RreferrerEmail"P
PurchaseReason>
type (2*.exa.seat_management_pb.PurchaseReasonTypeRtype"<
GetTeamCreditEntriesRequest

auth_token (	R	authToken"j
GetTeamCreditEntriesResponseJ
entries (20.exa.seat_management_pb.FlexCreditChronicleEntryRentries"j
SendPhoneVerificationRequest!
phone_number (	RphoneNumber'
turnstile_token (	RturnstileToken"
SendPhoneVerificationResponse"|
SendEmailVerificationRequest

first_name (	R	firstName
email (	Remail'
turnstile_token (	RturnstileToken"
SendEmailVerificationResponse"”
UpdateTeamConfigRequest
secret (	Rsecret
	team_name (	RteamName3
user_prompt_credit_cap (RuserPromptCreditCap/
user_flow_credit_cap (RuserFlowCreditCap0
disable_prompt_limit (RdisablePromptLimit,
disable_flow_limit (RdisableFlowLimit"
UpdateTeamConfigResponse"ê
UpdateTeamConfigExternalRequest

auth_token (	R	authToken:
allow_auto_run_commands (H RallowAutoRunCommandsˆ/
allow_mcp_servers (HRallowMcpServersˆ7
allow_app_deployments (HRallowAppDeploymentsˆ5
allow_github_reviews (HRallowGithubReviewsˆH
allow_github_description_edits (HRallowGithubDescriptionEditsˆH
pull_request_review_guidelines (	HRpullRequestReviewGuidelinesˆR
#pull_request_description_guidelines (	HR pullRequestDescriptionGuidelinesˆL
 allow_individual_level_analytics	 (HRallowIndividualLevelAnalyticsˆA
allow_conversation_sharing
 (HRallowConversationSharingˆF
allow_sandbox_app_deployments (H	RallowSandboxAppDeploymentsˆB
allow_teams_app_deployments (H
RallowTeamsAppDeploymentsˆ0
allow_attribution (HRallowAttributionˆ3
allowed_mcp_servers (	HRallowedMcpServersˆ>
allow_github_auto_reviews (HRallowGithubAutoReviewsˆR
#allow_browser_experimental_features (HR allowBrowserExperimentalFeaturesˆB
_allow_auto_run_commandsB
_allow_mcp_serversB
_allow_app_deploymentsB
_allow_github_reviewsB!
_allow_github_description_editsB!
_pull_request_review_guidelinesB&
$_pull_request_description_guidelinesB#
!_allow_individual_level_analyticsB
_allow_conversation_sharingB 
_allow_sandbox_app_deploymentsB
_allow_teams_app_deploymentsB
_allow_attributionB
_allowed_mcp_serversB
_allow_github_auto_reviewsB&
$_allow_browser_experimental_features""
 UpdateTeamConfigExternalResponse";
GetTeamConfigRecordRequest

auth_token (	R	authToken"a
GetTeamConfigRecordResponseB
team_config (2!.exa.codeium_common_pb.TeamConfigR
teamConfig"I
GetTeamInfoRequest
secret (	Rsecret
	team_name (	RteamName"´
UserInfo
api_key (	RapiKey
name (	Rname
email (	Remail;
signup_time (2.google.protobuf.TimestampR
signupTime

flows_used (R	flowsUsed!
prompts_used (RpromptsUsed'
num_acceptances (RnumAcceptances,
num_lines_accepted (RnumLinesAcceptedD
last_update_time	 (2.google.protobuf.TimestampRlastUpdateTimeW
first_antigravity_use_time
 (2.google.protobuf.TimestampRfirstAntigravityUseTimeF
team_status (2%.exa.codeium_common_pb.UserTeamStatusR
teamStatus(
has_cascade_seat (RhasCascadeSeat"¦
GetTeamInfoResponse0
team (2.exa.seat_management_pb.TeamRteam0
users_with_cascade (	BRusersWithCascade&
users_on_team (	BRusersOnTeam?

user_infos (2 .exa.seat_management_pb.UserInfoR	userInfosB
team_config (2!.exa.codeium_common_pb.TeamConfigR
teamConfig"A
IsValidReferralCodeRequest#
referral_code (	RreferralCode"B
IsValidReferralCodeResponse#
referrer_name (	RreferrerName"`
ProcessReferralCodeRequest

auth_token (	R	authToken#
referral_code (	RreferralCode"
ProcessReferralCodeResponse"ë
UserCascadeDetails
auth_uid (	RauthUid.
used_prompt_credits (RusedPromptCredits*
used_flow_credits (RusedFlowCredits^
free_cascade_usage_month_start (2.google.protobuf.TimestampRfreeCascadeUsageMonthStart"È
"AddExtraFlexCreditsInternalRequest
secret (	Rsecret
team_id (	RteamId
amount (RamountC
type (2/.exa.seat_management_pb.FlexCreditChronicleTypeRtype
email (	Remail"ƒ
#AddExtraFlexCreditsInternalResponse2
previous_flex_credits (RpreviousFlexCredits(
new_flex_credits (RnewFlexCredits"H
GetOneTimeAuthTokenRequest*
firebase_id_token (	RfirebaseIdToken"<
GetOneTimeAuthTokenResponse

auth_token (	R	authToken"A
 GetSetUserApiProviderKeysRequest

auth_token (	R	authToken"e
!GetSetUserApiProviderKeysResponse@
	providers (2".exa.codeium_common_pb.APIProviderR	providers"§
SetUserApiProviderKeyRequest

auth_token (	R	authToken>
provider (2".exa.codeium_common_pb.APIProviderRprovider(
provider_api_key (	RproviderApiKey"
SetUserApiProviderKeyResponse"€
DeleteUserApiProviderKeyRequest

auth_token (	R	authToken>
provider (2".exa.codeium_common_pb.APIProviderRprovider""
 DeleteUserApiProviderKeyResponse"K
DeleteApiKeyRequest

auth_token (	R	authToken
key_id (	RkeyId"
DeleteApiKeyResponse"8
GetApiKeySummaryRequest

auth_token (	R	authToken"Ê
ApiKeyForDisplay
key_id (	RkeyId&
key_for_display (	RkeyForDisplay9

created_at (2.google.protobuf.TimestampR	createdAt<
last_used_at (2.google.protobuf.TimestampR
lastUsedAt"_
GetApiKeySummaryResponseC
api_keys (2(.exa.seat_management_pb.ApiKeyForDisplayRapiKeys"%
LogOutUserRequest
uid (	Ruid"
LogOutUserResponse"@

TeamDomain
domain (	Rdomain
verified (Rverified"M
AddTeamDomainRequest

auth_token (	R	authToken
domain (	Rdomain"F
AddTeamDomainResponse-
verification_token (	RverificationToken"7
ListTeamDomainsRequest

auth_token (	R	authToken"`
ListTeamDomainsResponseE
team_domains (2".exa.seat_management_pb.TeamDomainRteamDomains"P
VerifyTeamDomainRequest

auth_token (	R	authToken
domain (	Rdomain"
VerifyTeamDomainResponse"P
DeleteTeamDomainRequest

auth_token (	R	authToken
domain (	Rdomain"
DeleteTeamDomainResponse"g
AddTeamDomainInternalRequest
secret (	Rsecret
team_id (	RteamId
domain (	Rdomain"
AddTeamDomainInternalResponse"j
DeleteTeamDomainInternalRequest
secret (	Rsecret
team_id (	RteamId
domain (	Rdomain""
 DeleteTeamDomainInternalResponse"Q
ListTeamDomainsInternalRequest
secret (	Rsecret
team_id (	RteamId"h
ListTeamDomainsInternalResponseE
team_domains (2".exa.seat_management_pb.TeamDomainRteamDomains"
RegisterGdmUserRequest"e
RegisterGdmUserResponse
created (Rcreated0
user (2.exa.seat_management_pb.UserRuser"t
AcceptTermsOfServiceRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata
version (	Rversion"
AcceptTermsOfServiceResponse"W
GetTermsOfServiceRequest;
metadata (2.exa.codeium_common_pb.MetadataRmetadata"h
GetTermsOfServiceResponse1
terms_of_service_text (	RtermsOfServiceText
version (	Rversion"‚
 UpdateUserAccountSettingsRequest^
user_account_settings (2*.exa.codeium_common_pb.UserAccountSettingsRuserAccountSettings"#
!UpdateUserAccountSettingsResponse*b
PaymentPeriod
PAYMENT_PERIOD_UNSPECIFIED 
PAYMENT_PERIOD_MONTH
PAYMENT_PERIOD_YEAR*Z
SubInterval
SUB_INTERVAL_UNSPECIFIED 
SUB_INTERVAL_MONTH
SUB_INTERVAL_YEAR*j
StripePrice
STRIPE_PRICE_UNSPECIFIED 
STRIPE_PRICE_TEAMS_MONTHLY
STRIPE_PRICE_TEAMS_YEARLY*q
CascadeSeatType!
CASCADE_SEAT_TYPE_UNSPECIFIED 
CASCADE_SEAT_TYPE_ENTRY
CASCADE_SEAT_TYPE_STANDARD*f
UserTeamDetailsType&
"USER_TEAM_DETAILS_TYPE_UNSPECIFIED '
#USER_TEAM_DETAILS_TYPE_CASCADE_SEAT*g
PendingTransactionType(
$PENDING_TRANSACTION_TYPE_UNSPECIFIED #
PENDING_TRANSACTION_TYPE_TOP_UP*°
FlexCreditChronicleType*
&FLEX_CREDIT_CHRONICLE_TYPE_UNSPECIFIED '
#FLEX_CREDIT_CHRONICLE_TYPE_PURCHASE'
#FLEX_CREDIT_CHRONICLE_TYPE_REFERRER$
 FLEX_CREDIT_CHRONICLE_TYPE_AVERY%
!FLEX_CREDIT_CHRONICLE_TYPE_TOP_UP&
"FLEX_CREDIT_CHRONICLE_TYPE_SUPPORT"
FLEX_CREDIT_CHRONICLE_TYPE_GTM*|
PurchaseReasonType$
 PURCHASE_REASON_TYPE_UNSPECIFIED 
PURCHASE_REASON_TYPE_MANUAL
PURCHASE_REASON_TYPE_TOP_UP2·_
SeatManagementServicen
GetUserStatus,.exa.seat_management_pb.GetUserStatusRequest-.exa.seat_management_pb.GetUserStatusResponse" _
GetUsers'.exa.seat_management_pb.GetUsersRequest(.exa.seat_management_pb.GetUsersResponse" t
RegisterGdmUser..exa.seat_management_pb.RegisterGdmUserRequest/.exa.seat_management_pb.RegisterGdmUserResponse" t
GetRolesForUser..exa.seat_management_pb.GetRolesForUserRequest/.exa.seat_management_pb.GetRolesForUserResponse" h
AddUserRole*.exa.seat_management_pb.AddUserRoleRequest+.exa.seat_management_pb.AddUserRoleResponse" q
RemoveUserRole-.exa.seat_management_pb.RemoveUserRoleRequest..exa.seat_management_pb.RemoveUserRoleResponse" k
RegisterUser+.exa.seat_management_pb.RegisterUserRequest,.exa.seat_management_pb.RegisterUserResponse" n
MigrateApiKey,.exa.seat_management_pb.MigrateApiKeyRequest-.exa.seat_management_pb.MigrateApiKeyResponse" €
GetOneTimeAuthToken2.exa.seat_management_pb.GetOneTimeAuthTokenRequest3.exa.seat_management_pb.GetOneTimeAuthTokenResponse" e

UpdateName).exa.seat_management_pb.UpdateNameRequest*.exa.seat_management_pb.UpdateNameResponse" €
UpdateInboundSource2.exa.seat_management_pb.UpdateInboundSourceRequest3.exa.seat_management_pb.UpdateInboundSourceResponse" °
#GetProfilePicturePresignedUploadUrlB.exa.seat_management_pb.GetProfilePicturePresignedUploadUrlRequestC.exa.seat_management_pb.GetProfilePicturePresignedUploadUrlResponse" ›
ProfilePictureUploadComplete;.exa.seat_management_pb.ProfilePictureUploadCompleteRequest<.exa.seat_management_pb.ProfilePictureUploadCompleteResponse" q
GetProfileData-.exa.seat_management_pb.GetProfileDataRequest..exa.seat_management_pb.GetProfileDataResponse" ƒ
DeleteProfilePicture3.exa.seat_management_pb.DeleteProfilePictureRequest4.exa.seat_management_pb.DeleteProfilePictureResponse" e

DeleteUser).exa.seat_management_pb.DeleteUserRequest*.exa.seat_management_pb.DeleteUserResponse" q
GetCurrentUser-.exa.seat_management_pb.GetCurrentUserRequest..exa.seat_management_pb.GetCurrentUserResponse" z
RequestTeamAccess0.exa.seat_management_pb.RequestTeamAccessRequest1.exa.seat_management_pb.RequestTeamAccessResponse" t
GetTeamMetadata..exa.seat_management_pb.GetTeamMetadataRequest/.exa.seat_management_pb.GetTeamMetadataResponse" }
RemoveUserFromTeam1.exa.seat_management_pb.RemoveUserFromTeamRequest2.exa.seat_management_pb.RemoveUserFromTeamResponse" €
RefreshTeamInviteId2.exa.seat_management_pb.RefreshTeamInviteIdRequest3.exa.seat_management_pb.RefreshTeamInviteIdResponse" w
GrantPreapproval/.exa.seat_management_pb.GrantPreapprovalRequest0.exa.seat_management_pb.GrantPreapprovalResponse" z
RevokePreapproval0.exa.seat_management_pb.RevokePreapprovalRequest1.exa.seat_management_pb.RevokePreapprovalResponse" t
GetPreapprovals..exa.seat_management_pb.GetPreapprovalsRequest/.exa.seat_management_pb.GetPreapprovalsResponse" ‰
GetPreapprovalMetadata5.exa.seat_management_pb.GetPreapprovalMetadataRequest6.exa.seat_management_pb.GetPreapprovalMetadataResponse" z
RejectPreapproval0.exa.seat_management_pb.RejectPreapprovalRequest1.exa.seat_management_pb.RejectPreapprovalResponse" †
GetPreapprovalForUser4.exa.seat_management_pb.GetPreapprovalForUserRequest5.exa.seat_management_pb.GetPreapprovalForUserResponse" €
GetUserSubscription2.exa.seat_management_pb.GetUserSubscriptionRequest3.exa.seat_management_pb.GetUserSubscriptionResponse" €
IsUsernameAvailable2.exa.seat_management_pb.IsUsernameAvailableRequest3.exa.seat_management_pb.IsUsernameAvailableResponse" q
UpdateUsername-.exa.seat_management_pb.UpdateUsernameRequest..exa.seat_management_pb.UpdateUsernameResponse" w
UpdateOccupation/.exa.seat_management_pb.UpdateOccupationRequest0.exa.seat_management_pb.UpdateOccupationResponse" n
UpdateProfile,.exa.seat_management_pb.UpdateProfileRequest-.exa.seat_management_pb.UpdateProfileResponse" Œ
UpdateProfileVisibility6.exa.seat_management_pb.UpdateProfileVisibilityRequest7.exa.seat_management_pb.UpdateProfileVisibilityResponse" w
GetPublicProfile/.exa.seat_management_pb.GetPublicProfileRequest0.exa.seat_management_pb.GetPublicProfileResponse" q
GetSSOProvider-.exa.seat_management_pb.GetSSOProviderRequest..exa.seat_management_pb.GetSSOProviderResponse" t
SaveSSOProvider..exa.seat_management_pb.SaveSSOProviderRequest/.exa.seat_management_pb.SaveSSOProviderResponse" ƒ
UserSSOLoginRedirect3.exa.seat_management_pb.UserSSOLoginRedirectRequest4.exa.seat_management_pb.UserSSOLoginRedirectResponse" †
SendPhoneVerification4.exa.seat_management_pb.SendPhoneVerificationRequest5.exa.seat_management_pb.SendPhoneVerificationResponse" †
SendEmailVerification4.exa.seat_management_pb.SendEmailVerificationRequest5.exa.seat_management_pb.SendEmailVerificationResponse" e

DeleteTeam).exa.seat_management_pb.DeleteTeamRequest*.exa.seat_management_pb.DeleteTeamResponse" w
GetTeamsFeatures/.exa.seat_management_pb.GetTeamsFeaturesRequest0.exa.seat_management_pb.GetTeamsFeaturesResponse" b
	GetGroups(.exa.seat_management_pb.GetGroupsRequest).exa.seat_management_pb.GetGroupsResponse" ƒ
GetUserNotifications3.exa.seat_management_pb.GetUserNotificationsRequest4.exa.seat_management_pb.GetUserNotificationsResponse" €
GetCascadeAnalytics2.exa.seat_management_pb.GetCascadeAnalyticsRequest3.exa.seat_management_pb.GetCascadeAnalyticsResponse" ƒ
GetTeamCreditEntries3.exa.seat_management_pb.GetTeamCreditEntriesRequest4.exa.seat_management_pb.GetTeamCreditEntriesResponse" ƒ
ConnectGithubAccount3.exa.seat_management_pb.ConnectGithubAccountRequest4.exa.seat_management_pb.ConnectGithubAccountResponse" Œ
DisconnectGithubAccount6.exa.seat_management_pb.DisconnectGithubAccountRequest7.exa.seat_management_pb.DisconnectGithubAccountResponse" ‰
GetGitHubAccountStatus5.exa.seat_management_pb.GetGitHubAccountStatusRequest6.exa.seat_management_pb.GetGitHubAccountStatusResponse" ƒ
GetGitHubAccessToken3.exa.seat_management_pb.GetGitHubAccessTokenRequest4.exa.seat_management_pb.GetGitHubAccessTokenResponse" †
ConnectNetlifyAccount4.exa.seat_management_pb.ConnectNetlifyAccountRequest5.exa.seat_management_pb.ConnectNetlifyAccountResponse" 
DisconnectNetlifyAccount7.exa.seat_management_pb.DisconnectNetlifyAccountRequest8.exa.seat_management_pb.DisconnectNetlifyAccountResponse" Œ
GetNetlifyAccountStatus6.exa.seat_management_pb.GetNetlifyAccountStatusRequest7.exa.seat_management_pb.GetNetlifyAccountStatusResponse" €
CreateTeamApiSecret2.exa.seat_management_pb.CreateTeamApiSecretRequest3.exa.seat_management_pb.CreateTeamApiSecretResponse" €
UpdateTeamApiSecret2.exa.seat_management_pb.UpdateTeamApiSecretRequest3.exa.seat_management_pb.UpdateTeamApiSecretResponse" ƒ
GetAllTeamApiSecrets3.exa.seat_management_pb.GetAllTeamApiSecretsRequest4.exa.seat_management_pb.GetAllTeamApiSecretsResponse" €
DeleteTeamApiSecret2.exa.seat_management_pb.DeleteTeamApiSecretRequest3.exa.seat_management_pb.DeleteTeamApiSecretResponse" n
GetPlanStatus,.exa.seat_management_pb.GetPlanStatusRequest-.exa.seat_management_pb.GetPlanStatusResponse" e

CreateRole).exa.seat_management_pb.CreateRoleRequest*.exa.seat_management_pb.CreateRoleResponse" e

DeleteRole).exa.seat_management_pb.DeleteRoleRequest*.exa.seat_management_pb.DeleteRoleResponse" e

UpdateRole).exa.seat_management_pb.UpdateRoleRequest*.exa.seat_management_pb.UpdateRoleResponse" _
GetRoles'.exa.seat_management_pb.GetRolesRequest(.exa.seat_management_pb.GetRolesResponse" t
UpdateUserRoles..exa.seat_management_pb.UpdateUserRolesRequest/.exa.seat_management_pb.UpdateUserRolesResponse" €
BulkUpdateUserRoles2.exa.seat_management_pb.BulkUpdateUserRolesRequest3.exa.seat_management_pb.BulkUpdateUserRolesResponse" 
GetTeamsFeaturesInternal7.exa.seat_management_pb.GetTeamsFeaturesInternalRequest8.exa.seat_management_pb.GetTeamsFeaturesInternalResponse" ˜
UpdateTeamsFeaturesInternal:.exa.seat_management_pb.UpdateTeamsFeaturesInternalRequest;.exa.seat_management_pb.UpdateTeamsFeaturesInternalResponse" w
SetTeamsFeatures/.exa.seat_management_pb.SetTeamsFeaturesRequest0.exa.seat_management_pb.SetTeamsFeaturesResponse" •
UpdateUserFeaturesInternal9.exa.seat_management_pb.UpdateUserFeaturesInternalRequest:.exa.seat_management_pb.UpdateUserFeaturesInternalResponse" †
BulkEditUserApprovals4.exa.seat_management_pb.BulkEditUserApprovalsRequest5.exa.seat_management_pb.BulkEditUserApprovalsResponse" ž
BulkEditUserApprovalsInternal<.exa.seat_management_pb.BulkEditUserApprovalsInternalRequest=.exa.seat_management_pb.BulkEditUserApprovalsInternalResponse" w
UpdateTeamConfig/.exa.seat_management_pb.UpdateTeamConfigRequest0.exa.seat_management_pb.UpdateTeamConfigResponse" h
GetTeamInfo*.exa.seat_management_pb.GetTeamInfoRequest+.exa.seat_management_pb.GetTeamInfoResponse" 
UpdateTeamConfigExternal7.exa.seat_management_pb.UpdateTeamConfigExternalRequest8.exa.seat_management_pb.UpdateTeamConfigExternalResponse" €
GetTeamConfigRecord2.exa.seat_management_pb.GetTeamConfigRecordRequest3.exa.seat_management_pb.GetTeamConfigRecordResponse" t
GetTeamSettings..exa.seat_management_pb.GetTeamSettingsRequest/.exa.seat_management_pb.GetTeamSettingsResponse" ž
UpdateCascadeWebSearchEnabled<.exa.seat_management_pb.UpdateCascadeWebSearchEnabledRequest=.exa.seat_management_pb.UpdateCascadeWebSearchEnabledResponse" h
GetMucsInfo*.exa.seat_management_pb.GetMucsInfoRequest+.exa.seat_management_pb.GetMucsInfoResponse" q
GetWrapped2024-.exa.seat_management_pb.GetWrapped2024Request..exa.seat_management_pb.GetWrapped2024Response" €
IsValidReferralCode2.exa.seat_management_pb.IsValidReferralCodeRequest3.exa.seat_management_pb.IsValidReferralCodeResponse" €
ProcessReferralCode2.exa.seat_management_pb.ProcessReferralCodeRequest3.exa.seat_management_pb.ProcessReferralCodeResponse" ˜
AddExtraFlexCreditsInternal:.exa.seat_management_pb.AddExtraFlexCreditsInternalRequest;.exa.seat_management_pb.AddExtraFlexCreditsInternalResponse" ’
GetSetUserApiProviderKeys8.exa.seat_management_pb.GetSetUserApiProviderKeysRequest9.exa.seat_management_pb.GetSetUserApiProviderKeysResponse" †
SetUserApiProviderKey4.exa.seat_management_pb.SetUserApiProviderKeyRequest5.exa.seat_management_pb.SetUserApiProviderKeyResponse" 
DeleteUserApiProviderKey7.exa.seat_management_pb.DeleteUserApiProviderKeyRequest8.exa.seat_management_pb.DeleteUserApiProviderKeyResponse" w
GetApiKeySummary/.exa.seat_management_pb.GetApiKeySummaryRequest0.exa.seat_management_pb.GetApiKeySummaryResponse" k
DeleteApiKey+.exa.seat_management_pb.DeleteApiKeyRequest,.exa.seat_management_pb.DeleteApiKeyResponse" e

LogOutUser).exa.seat_management_pb.LogOutUserRequest*.exa.seat_management_pb.LogOutUserResponse" n
AddTeamDomain,.exa.seat_management_pb.AddTeamDomainRequest-.exa.seat_management_pb.AddTeamDomainResponse" t
ListTeamDomains..exa.seat_management_pb.ListTeamDomainsRequest/.exa.seat_management_pb.ListTeamDomainsResponse" w
VerifyTeamDomain/.exa.seat_management_pb.VerifyTeamDomainRequest0.exa.seat_management_pb.VerifyTeamDomainResponse" w
DeleteTeamDomain/.exa.seat_management_pb.DeleteTeamDomainRequest0.exa.seat_management_pb.DeleteTeamDomainResponse" †
AddTeamDomainInternal4.exa.seat_management_pb.AddTeamDomainInternalRequest5.exa.seat_management_pb.AddTeamDomainInternalResponse" 
DeleteTeamDomainInternal7.exa.seat_management_pb.DeleteTeamDomainInternalRequest8.exa.seat_management_pb.DeleteTeamDomainInternalResponse" Œ
ListTeamDomainsInternal6.exa.seat_management_pb.ListTeamDomainsInternalRequest7.exa.seat_management_pb.ListTeamDomainsInternalResponse" ƒ
AcceptTermsOfService3.exa.seat_management_pb.AcceptTermsOfServiceRequest4.exa.seat_management_pb.AcceptTermsOfServiceResponse" z
GetTermsOfService0.exa.seat_management_pb.GetTermsOfServiceRequest1.exa.seat_management_pb.GetTermsOfServiceResponse" ’
UpdateUserAccountSettings8.exa.seat_management_pb.UpdateUserAccountSettingsRequest9.exa.seat_management_pb.UpdateUserAccountSettingsResponse" BòOPEN_TO_OPAQUE_HYBRIDbproto3
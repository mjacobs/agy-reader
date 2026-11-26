
4third_party/jetski/cortex/policyguardian/state.protojetski.cortex.policyguardiangoogle/protobuf/struct.proto"Ç
DeniedAction
	tool_name (	RtoolName+
args (2.google.protobuf.StructRargs
reason (	Rreason'
proctor_flagged (RproctorFlagged,
proctor_would_stop (RproctorWouldStop"¬
EnvState/
untrusted_downloads (	RuntrustedDownloadsP
symlinks (24.jetski.cortex.policyguardian.EnvState.SymlinksEntryRsymlinks%
modified_files (	RmodifiedFilesW
env_changes (26.jetski.cortex.policyguardian.EnvState.EnvChangesEntryR
envChanges)
executable_files (	RexecutableFilesQ
denied_actions (2*.jetski.cortex.policyguardian.DeniedActionRdeniedActions#
created_files (	RcreatedFiles;
SymlinksEntry
key (	Rkey
value (	Rvalue:8=
EnvChangesEntry
key (	Rkey
value (	Rvalue:8"ô
PendingEnvUpdates/
untrusted_downloads (	RuntrustedDownloadsY
symlinks (2=.jetski.cortex.policyguardian.PendingEnvUpdates.SymlinksEntryRsymlinks%
modified_files (	RmodifiedFiles`
env_changes (2?.jetski.cortex.policyguardian.PendingEnvUpdates.EnvChangesEntryR
envChanges)
executable_files (	RexecutableFiles#
created_files (	RcreatedFiles;
SymlinksEntry
key (	Rkey
value (	Rvalue:8=
EnvChangesEntry
key (	Rkey
value (	Rvalue:8"f
UserTurn
index (Rindex
text (	Rtext0
prior_assistant_text (	RpriorAssistantText"â
QuarantinedTarget
target (	Rtarget
category (	Rcategory
reason (	Rreason
rule (	Rrule
count (Rcount

first_step (R	firstStep
	last_step (RlastStep
	last_call (RlastCall"e
ProctorStrike
step (Rstep
tool (	Rtool
rule (	Rrule
targets (	Rtargets"Ì
ProctorSession
calls (RcallsO

quarantine (2/.jetski.cortex.policyguardian.QuarantinedTargetR
quarantine
strikes (RstrikesJ

strike_log (2+.jetski.cortex.policyguardian.ProctorStrikeR	strikeLog

locked_out (R	lockedOut&
locked_out_step (RlockedOutStep)
guardian_denials (RguardianDenials.
elevated_until_call (RelevatedUntilCallE
denials	 (2+.jetski.cortex.policyguardian.ProctorDenialRdenials9
last_alignment_check_read
 (RlastAlignmentCheckReadK

mcp_drifts (2,.jetski.cortex.policyguardian.McpDriftRecordR	mcpDrifts]
skill_risks (2<.jetski.cortex.policyguardian.ProctorSession.SkillRisksEntryR
skillRisks0
active_denial_streak (RactiveDenialStreakk
SkillRisksEntry
key (	RkeyB
value (2,.jetski.cortex.policyguardian.SkillRiskEntryRvalue:8"Q
SkillRiskEntry
signals (	Rsignals%
shadow_tainted (RshadowTainted"†
McpDriftRecord
	tool_name (	RtoolName

drift_type (	R	driftType
reason (	Rreason 
destructive (Rdestructive"¢
ProctorDenial
step (Rstep
tool (	Rtool
target (	Rtarget
reason (	Rreason'
attack_category (	RattackCategory
ask (Rask"è
ProctorState
version (RversionH
enforced (2,.jetski.cortex.policyguardian.ProctorSessionRenforcedD
shadow (2,.jetski.cortex.policyguardian.ProctorSessionRshadow.
shadow_denied_steps (RshadowDeniedSteps"›
GuardianState<
state (2&.jetski.cortex.policyguardian.EnvStateRstateb
pending_state (2=.jetski.cortex.policyguardian.GuardianState.PendingStateEntryRpendingState!
user_prompts (	RuserPromptsE

user_turns (2&.jetski.cortex.policyguardian.UserTurnR	userTurns&
user_turn_count (RuserTurnCount

provenance (R
provenanceD
proctor (2*.jetski.cortex.policyguardian.ProctorStateRproctorp
PendingStateEntry
key (RkeyE
value (2/.jetski.cortex.policyguardian.PendingEnvUpdatesRvalue:8BAZ?google3/third_party/jetski/cortex/policyguardian/state_go_protobproto3
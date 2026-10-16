
4third_party/jetski/cortex/policyguardian/state.protojetski.cortex.policyguardiangoogle/protobuf/struct.proto"p
DeniedAction
	tool_name (	RtoolName+
args (2.google.protobuf.StructRargs
reason (	Rreason"¬
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
value (	Rvalue:8"4
UserTurn
index (Rindex
text (	Rtext"µ
GuardianState<
state (2&.jetski.cortex.policyguardian.EnvStateRstateb
pending_state (2=.jetski.cortex.policyguardian.GuardianState.PendingStateEntryRpendingState!
user_prompts (	RuserPromptsE

user_turns (2&.jetski.cortex.policyguardian.UserTurnR	userTurns&
user_turn_count (RuserTurnCountp
PendingStateEntry
key (RkeyE
value (2/.jetski.cortex.policyguardian.PendingEnvUpdatesRvalue:8BAZ?google3/third_party/jetski/cortex/policyguardian/state_go_protobproto3
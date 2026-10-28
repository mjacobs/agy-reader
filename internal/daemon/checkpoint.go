package daemon

// CapturedUserRequest is a checkpoint's copy of user text and attached media
// (agy 1.4.6). Legacy checkpoints store only Checkpoint.UserRequests.
type CapturedUserRequest struct {
	Text  string  `json:"text,omitempty"`
	Media []Media `json:"media,omitempty"`
}

// CheckpointMedia is the older checkpoint-specific name for shared media.
type CheckpointMedia = Media

// CheckpointBlobref is the older checkpoint-specific name for a media reference.
type CheckpointBlobref = MediaBlobref

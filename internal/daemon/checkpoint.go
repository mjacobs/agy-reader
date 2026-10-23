package daemon

// CapturedUserRequest is a checkpoint's copy of user text and attached media
// (agy 1.4.6). Legacy checkpoints store only Checkpoint.UserRequests.
type CapturedUserRequest struct {
	Text  string            `json:"text,omitempty"`
	Media []CheckpointMedia `json:"media,omitempty"`
}

// CheckpointMedia is the displayable view of codeium_common_pb.Media. Raw
// trajectory sidecars retain the complete message. Embedded bytes are retained
// here only to describe their presence; Markdown must never print them.
type CheckpointMedia struct {
	MimeType    string             `json:"mimeType,omitempty"`
	Description string             `json:"description,omitempty"`
	InlineData  string             `json:"inlineData,omitempty"`
	Blobref     *CheckpointBlobref `json:"blobref,omitempty"`
	URI         string             `json:"uri,omitempty"`
	Thumbnail   string             `json:"thumbnail,omitempty"`
	MediaID     string             `json:"mediaId,omitempty"`
	DisplayName string             `json:"displayName,omitempty"`
	StartPage   *int32             `json:"startPage,omitempty"`
	EndPage     *int32             `json:"endPage,omitempty"`
}

// CheckpointBlobref identifies stored media when no URI was captured. It does
// not imply that this reader can fetch the attachment.
type CheckpointBlobref struct {
	BlobID string `json:"blobId,omitempty"`
}

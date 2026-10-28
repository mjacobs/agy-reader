package daemon

// Media is the displayable view of codeium_common_pb.Media, used by user
// requests, planner responses and checkpoints. Raw trajectory sidecars retain
// the complete message. Embedded bytes are retained here only to describe
// their presence; Markdown must never print them.
type Media struct {
	MimeType    string        `json:"mimeType,omitempty"`
	Description string        `json:"description,omitempty"`
	InlineData  string        `json:"inlineData,omitempty"`
	Blobref     *MediaBlobref `json:"blobref,omitempty"`
	URI         string        `json:"uri,omitempty"`
	Thumbnail   string        `json:"thumbnail,omitempty"`
	MediaID     string        `json:"mediaId,omitempty"`
	DisplayName string        `json:"displayName,omitempty"`
	StartPage   *int32        `json:"startPage,omitempty"`
	EndPage     *int32        `json:"endPage,omitempty"`
}

// MediaBlobref identifies stored media when no URI was captured. It does not
// imply that this reader can fetch the attachment.
type MediaBlobref struct {
	BlobID string `json:"blobId,omitempty"`
}

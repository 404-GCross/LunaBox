package batchupload

// Item describes one local file and its destination in a cloud provider.
type Item struct {
	CloudPath string
	LocalPath string
}

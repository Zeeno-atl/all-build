package tasks

// A list of task types.
const (
	TypeCompileFile = "compile"
)

type File struct {
	Path    string `json:"path"`
	Chmod   int    `json:"chmod"`
	Content []byte `json:"content"`
}

type Response struct {
	ReturnCode int    `json:"returnCode"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Files      []File `json:"files"`
}

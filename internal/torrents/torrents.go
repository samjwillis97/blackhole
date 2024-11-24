package torrents

import "path"

type ToProcess struct {
	path     string
	filename string
}

func New(filePath string) ToProcess {
	_, filename := path.Split(filePath)
	return ToProcess{
		path:     filePath,
		filename: filename,
	}
}

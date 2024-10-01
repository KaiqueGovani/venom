package model

type Project struct {
	Name         string            `json:"name"`
	FileName     string            `json:"file_name"`
	TargetFolder string            `json:"target_folder"`
	Variables    map[string]string `json:"variables"`
}

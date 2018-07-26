package util

//Config contains rolling upgrade configuration parameters
type Config struct {
	Namespace    string
	ResourceName string
	Annotation   string
	SHAValue     string
}

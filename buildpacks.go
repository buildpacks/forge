package forge

type Buildpack struct {
	Name       string
	URL        string
	VersionURL string
}

type SystemBuildpacks []Buildpack

func (b SystemBuildpacks) names() []string {
	var names []string
	for _, bp := range b {
		names = append(names, bp.Name)
	}
	return names
}
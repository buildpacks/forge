package forge

type Buildpack struct {
	Name       string
	URL        string
	VersionURL string
}

type Buildpacks []Buildpack

func (b Buildpacks) names() []string {
	var names []string
	for _, bp := range b {
		names = append(names, bp.Name)
	}
	return names
}

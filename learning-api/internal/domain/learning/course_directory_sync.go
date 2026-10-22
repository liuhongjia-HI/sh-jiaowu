package learning

type CourseDirectorySyncRequest struct {
	IncludeMaterials *bool                                         `json:"includeMaterials,omitempty"`
	Resolutions      map[string]map[string]DirectorySyncResolution `json:"resolutions,omitempty"`
	UnitIDs          []string                                      `json:"unitIds,omitempty"`
	SourceCourseID   string                                        `json:"sourceCourseId"`
	TargetCourseIDs  []string                                      `json:"targetCourseIds"`
	Snapshots        map[string]string                             `json:"snapshots,omitempty"`
}

type DirectorySyncResolution struct {
	TargetNodeID string   `json:"targetNodeId"`
	MergeNodeIDs []string `json:"mergeNodeIds,omitempty"`
}
type DirectorySyncCandidate struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}
type DirectorySyncConflict struct {
	AllowNew     bool                     `json:"allowNew"`
	SourceNodeID string                   `json:"sourceNodeId"`
	SourceLabel  string                   `json:"sourceLabel"`
	Candidates   []DirectorySyncCandidate `json:"candidates"`
	Resolved     bool                     `json:"resolved"`
}
type CourseDirectorySyncTarget struct {
	Conflicts          []DirectorySyncConflict `json:"conflicts,omitempty"`
	Merged             []string                `json:"merged,omitempty"`
	MaterialsAdded     []string                `json:"materialsAdded,omitempty"`
	MaterialsUpdated   []string                `json:"materialsUpdated,omitempty"`
	MaterialsPreserved int                     `json:"materialsPreserved"`

	CourseID   string   `json:"courseId"`
	CourseName string   `json:"courseName"`
	Added      []string `json:"added"`
	Updated    []string `json:"updated"`
	Preserved  int      `json:"preserved"`
	Snapshot   string   `json:"snapshot"`
	Error      string   `json:"error,omitempty"`
	Status     string   `json:"status,omitempty"`
}

type CourseDirectorySyncResult struct {
	Targets []CourseDirectorySyncTarget `json:"targets"`
}

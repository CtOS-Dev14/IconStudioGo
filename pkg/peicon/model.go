package peicon

// IconLayerInfo represents a single layer/resolution within an icon group
type IconLayerInfo struct {
	Width    int  `json:"width"`
	Height   int  `json:"height"`
	RawW     int  `json:"raw_w"`
	RawH     int  `json:"raw_h"`
	Colors   int  `json:"colors"`
	Planes   int  `json:"planes"`
	BitCount int  `json:"bit_count"`
	Size     int  `json:"size"`
	IconID   int  `json:"icon_id"`
	IsPNG    bool `json:"is_png"`
}

// IconGroupSummary represents an icon group displayed in the grid
type IconGroupSummary struct {
	GroupID     string          `json:"group_id"`
	TotalLayers int             `json:"total_layers"`
	HasUHD      bool            `json:"has_uhd"`
	MaxRes      int             `json:"max_res"`
	MaxBpp      int             `json:"max_bpp"`
	Thumbnail   string          `json:"thumbnail"` // Base64 data URL
	Layers      []IconLayerInfo `json:"layers"`
}

// ParseResult is returned after scanning a PE/MUN file
type ParseResult struct {
	FilePath    string             `json:"file_path"`
	FileName    string             `json:"file_name"`
	TotalGroups int                `json:"total_groups"`
	Groups      []IconGroupSummary `json:"groups"`
}

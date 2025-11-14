package probing

type MediaStreamInfoSideData struct {
	SideDataType              *string `json:"side_data_type"`
	DvVersionMajor            *int    `json:"dv_version_major"`
	DvVersionMinor            *int    `json:"dv_version_minor"`
	DvProfile                 *int    `json:"dv_profile"`
	DvLevel                   *int    `json:"dv_level"`
	RpuPresentFlag            *int    `json:"rpu_present_flag"`
	ElPresentFlag             *int    `json:"el_present_flag"`
	BlPresentFlag             *int    `json:"bl_present_flag"`
	DvBlSignalCompatibilityId *int    `json:"dv_bl_signal_compatibility_id"`
}

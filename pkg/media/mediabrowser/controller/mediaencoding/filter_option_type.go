package mediaencoding

type FilterOptionType int

const (
	ScaleCudaFormat FilterOptionType = iota
	TonemapCudaName
	TonemapOpenclBt2390
	OverlayOpenclFrameSync
	OverlayVaapiFrameSync
	OverlayVulkanFrameSync
	TransposeOpenclReversal
	OverlayOpenclAlphaFormat
	OverlayCudaAlphaFormat
)

func (f FilterOptionType) String() string {
	switch f {
	case ScaleCudaFormat:
		return "ScaleCudaFormat"
	case TonemapCudaName:
		return "TonemapCudaName"
	case TonemapOpenclBt2390:
		return "TonemapOpenclBt2390"
	case OverlayOpenclFrameSync:
		return "OverlayOpenclFrameSync"
	case OverlayVaapiFrameSync:
		return "OverlayVaapiFrameSync"
	case OverlayVulkanFrameSync:
		return "OverlayVulkanFrameSync"
	case TransposeOpenclReversal:
		return "TransposeOpenclReversal"
	case OverlayOpenclAlphaFormat:
		return "OverlayOpenclAlphaFormat"
	case OverlayCudaAlphaFormat:
		return "OverlayCudaAlphaFormat"
	default:
		return "Unknown"
	}
}

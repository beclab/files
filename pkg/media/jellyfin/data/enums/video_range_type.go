package enums

type VideoRangeType string

const (
	VideoRangeTypeUnknown             VideoRangeType = "Unknown"
	VideoRangeTypeSDR                 VideoRangeType = "SDR"
	VideoRangeTypeHDR10               VideoRangeType = "HDR10"
	VideoRangeTypeHLG                 VideoRangeType = "HLG"
	VideoRangeTypeDOVI                VideoRangeType = "DOVI"
	VideoRangeTypeDOVIWithHDR10       VideoRangeType = "DOVIWithHDR10"
	VideoRangeTypeDOVIWithHLG         VideoRangeType = "DOVIWithHLG"
	VideoRangeTypeDOVIWithSDR         VideoRangeType = "DOVIWithSDR"
	VideoRangeTypeDOVIWithEL          VideoRangeType = "DOVIWithEL"
	VideoRangeTypeDOVIWithHDR10Plus   VideoRangeType = "DOVIWithHDR10Plus"
	VideoRangeTypeDOVIWithELHDR10Plus VideoRangeType = "DOVIWithELHDR10Plus"
	VideoRangeTypeDOVIInvalid         VideoRangeType = "DOVIInvalid"
	VideoRangeTypeHDR10Plus           VideoRangeType = "HDR10Plus"
)

package mediaencoding

type BitStreamFilterOptionType int

const (
	BitStreamFilterOptionTypeHevcMetadataRemoveDovi      BitStreamFilterOptionType = 0
	BitStreamFilterOptionTypeHevcMetadataRemoveHdr10Plus BitStreamFilterOptionType = 1
	BitStreamFilterOptionTypeAv1MetadataRemoveDovi       BitStreamFilterOptionType = 2
	BitStreamFilterOptionTypeAv1MetadataRemoveHdr10Plus  BitStreamFilterOptionType = 3
	BitStreamFilterOptionTypeDoviRpuStrip                BitStreamFilterOptionType = 4
)

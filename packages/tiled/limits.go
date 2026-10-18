package tiled

import "errors"

const (
	MaxDocumentBytes        = 4 << 20
	MaxTotalDocumentBytes   = 16 << 20
	MaxDocuments            = 65
	MaxMapDimension         = 256
	MaxLayers               = 256
	MaxLayerDepth           = 16
	MaxSprites              = 65536
	MaxObjects              = 65536
	MaxAnimationFrames      = 64
	MaxTotalAnimationFrames = 262144
	MaxImageDimension       = 16384
	MaxCoordinate           = 1000000
)

var ErrLimit = errors.New("Tiled input exceeds supported bounds")

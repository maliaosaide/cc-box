// WebDAV XML 数据结构
package webdav

import (
	"encoding/xml"
	"errors"
)

// 常见错误
var (
	ErrNotFound = errors.New("文件不存在")
	ErrConflict = errors.New("ETag 冲突（乐观锁）")
)

// FileInfo 文件元信息
type FileInfo struct {
	Path  string
	IsDir bool
	Size  int64
	ETag  string
}

// multistatus WebDAV 207 响应根节点
type multistatus struct {
	XMLName   xml.Name      `xml:"multistatus"`
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href     string        `xml:"href"`
	PropStat []davPropStat `xml:"propstat"`
}

type davPropStat struct {
	Prop   davProp `xml:"prop"`
	Status string  `xml:"status"`
}

type davProp struct {
	ETag          string          `xml:"getetag"`
	ContentLength int64           `xml:"getcontentlength"`
	LastModified  string          `xml:"getlastmodified"`
	ResourceType  davResourceType `xml:"resourcetype"`
}

type davResourceType struct {
	Collection *struct{} `xml:"collection"`
}

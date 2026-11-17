package pkg

import (
	"net/http"
	"path"
	"strings"
)

func GetFileType(key string) string {
	ext := strings.ToLower(path.Ext(key))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg":
		return "image"
	case ".mp4", ".avi", ".mov", ".mkv":
		return "video"
	case ".mp3", ".wav", ".flac":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".txt", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return "document"
	case ".zip", ".rar", ".tar", ".gz", ".7z":
		return "archive"
	case ".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".java", ".c", ".cpp", ".rb", ".php":
		return "code"
	case ".apk":
		return "apk"
	case ".exe":
		return "exe"
	default:
		return "other"
	}
}

func DetectContentType(data []byte, ext string) string {
	contentType := http.DetectContentType(data)

	if contentType == "application/octet-stream" || contentType == "text/plain; charset=utf-8" {
		ext = strings.ToLower(ext)
		switch ext {
		case ".json":
			return "application/json"
		case ".xml":
			return "application/xml"
		case ".css":
			return "text/css"
		case ".js":
			return "application/javascript"
		case ".html", ".htm":
			return "text/html"
		case ".csv":
			return "text/csv"
		case ".md":
			return "text/markdown"
		case ".yaml", ".yml":
			return "application/yaml"
		case ".zip":
			return "application/zip"
		case ".tar":
			return "application/x-tar"
		case ".gz":
			return "application/gzip"
		case ".pdf":
			return "application/pdf"
		case ".apk":
			return "application/vnd.android.package-archive"
		case ".exe":
			return "application/x-msdownload"
		}
	}

	return contentType
}

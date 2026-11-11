// 画像のファイル: 置き場所の決まりと読み込み(docs/library.md の「画像」)。
//
// ライブラリはファイルを読まない。画像は呼ぶ側が渡す fs.FS(BuildOptions.Assets・
// CheckOptions.Assets)から読む。src は文書のファイルからの相対パスで、fs.FS の
// 根は文書のフォルダ(コマンドは os.DirFS(文書のフォルダ) を渡す)。
//
//   - 絶対パス・URL(https: など)・文書のフォルダの外(..)・\ を含むものは
//     image-path の誤り。
//   - 無い・読めないものは image-missing、PNG・JPEG でないものは image-format。
//
// 大きさ(画素)は標準ライブラリの image/png・image/jpeg の DecodeConfig で読む。

package boxloom

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // DecodeConfig に JPEG を教える
	_ "image/png"  // DecodeConfig に PNG を教える
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// 画像の誤りの種類(check の code と同じ名前)。
var (
	errImagePath    = errors.New("image-path")
	errImageMissing = errors.New("image-missing")
	errImageFormat  = errors.New("image-format")
)

// imageError は画像の誤り。Is で種類(errImagePath など)を見分ける。
type imageError struct {
	kind error
	msg  string
}

func (e *imageError) Error() string        { return e.msg }
func (e *imageError) Is(target error) bool { return target == e.kind }

// imageCode は誤りの種類の code(画像の誤りでなければ "")。
func imageCode(err error) string {
	for _, k := range []error{errImagePath, errImageMissing, errImageFormat} {
		if errors.Is(err, k) {
			return k.Error()
		}
	}
	return ""
}

var reURLScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// checkImageSrc は src の書き方を見る(ファイルは読まない)。
func checkImageSrc(src string) error {
	bad := func(why string) error {
		return &imageError{errImagePath, fmt.Sprintf("%q: %s (write a path relative to the deck file, e.g. images/plan.png)", src, why)}
	}
	switch {
	case src == "":
		return bad("empty path")
	case reURLScheme.MatchString(src):
		return bad("URLs are not allowed; put the image file next to the deck")
	case strings.HasPrefix(src, "/"):
		return bad("absolute paths are not allowed")
	case strings.Contains(src, `\`):
		return bad(`use / as the separator, not \`)
	}
	for _, seg := range strings.Split(src, "/") {
		if seg == ".." {
			return bad("paths outside the deck's folder (..) are not allowed")
		}
	}
	return nil
}

// imageAsset は読んだ画像1枚。
type imageAsset struct {
	Data   []byte
	Format string // "png" | "jpeg"
	W, H   int    // 画素
}

// aspect は幅÷高さ。
func (a *imageAsset) aspect() float64 { return float64(a.W) / float64(a.H) }

// loadImage は fsys から src を読み、形式と画素の大きさを調べる。
func loadImage(fsys fs.FS, src string) (*imageAsset, error) {
	if err := checkImageSrc(src); err != nil {
		return nil, err
	}
	if fsys == nil {
		return nil, &imageError{errImageMissing, fmt.Sprintf("%q: no image files were given (the library reads images only from BuildOptions.Assets / CheckOptions.Assets)", src)}
	}
	name := path.Clean(src)
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, &imageError{errImageMissing, fmt.Sprintf("%q: cannot read the image: %v", src, err)}
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, &imageError{errImageFormat, fmt.Sprintf("%q: not a PNG or JPEG image (Boxloom embeds PNG and JPEG only)", src)}
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, &imageError{errImageFormat, fmt.Sprintf("%q: the image has no pixels", src)}
	}
	return &imageAsset{Data: data, Format: format, W: cfg.Width, H: cfg.Height}, nil
}

// imageCache は1回の組み立ての中で同じ画像を2度読まないための表。
type imageCache struct {
	fsys fs.FS
	m    map[string]*imageAsset
}

func (c *imageCache) get(src string) (*imageAsset, error) {
	if c == nil {
		return loadImage(nil, src)
	}
	if a, ok := c.m[src]; ok {
		return a, nil
	}
	a, err := loadImage(c.fsys, src)
	if err != nil {
		return nil, err
	}
	if c.m == nil {
		c.m = map[string]*imageAsset{}
	}
	c.m[src] = a
	return a, nil
}

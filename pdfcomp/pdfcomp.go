package pdfcomp

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/create"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/primitives"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

var GlobDebug = false

// Compare two PDF files, and return true if they are visually the same.  Some messages
// may be printed to stderr.
// If images is set, will write png files highlighting the differences in each page.
// If pdf is given, will create PDF file highlighting bundling these images together.
// Resolution is the dpi to render images fo pages in the pdf for comparison.
// Highlighting is done with circles radius resolution / ratio.
// Does not check if resolution and ratio are sensible.  Try 150 and 30.
func EqualPDFs(file1, file2 string, images bool, pdf io.Writer, resolution, ratio int) (bool, error) {
	if file1 == file2 {
		if GlobDebug {
			fmt.Fprintf(os.Stderr, "two files are the same: %s\n", file1)
		}
		return true, nil
	}

	pages1, err := PageCount(file1)
	if err != nil {
		return false, fmt.Errorf("error getting page count for %s: %w", file1, err)
	}
	pages2, err := PageCount(file2)
	if err != nil {
		return false, fmt.Errorf("error getting page count for %s: %w", file2, err)
	}

	if pages1 != pages2 {
		if GlobDebug {
			fmt.Fprintf(os.Stderr, "two files have different numbers of pages, %s: %d, %s: %d\n", file1, pages1, file2, pages2)
		}
		if !images {
			return false, nil
		}
	}

	same := true
	pngFiles := []PageFile{}

	for i := range pages1 {
		page := i + 1
		// Get a PPM in memmory to work with
		ppm1, err := PdfToPPM(file1, page, resolution)
		if err != nil {
			return false, err
		}

		ppm2, err := PdfToPPM(file2, page, resolution)
		if err != nil {
			return false, err
		}

		// Convert to matrices for easier manipulation
		mat1, err := ppmToMatrix(ppm1)
		if err != nil {
			return false, err
		}

		mat2, err := ppmToMatrix(ppm2)
		if err != nil {
			return false, err
		}

		// Finally do some comparing
		thisSame, diff, err := equalImgMatrix(mat1, mat2, images || (pdf != nil))
		same = same && thisSame
		if err != nil {
			return false, err
		}

		if !same && (images || (pdf != nil)) {
			img1 := diffImage(mat1, diff, resolution/ratio)
			img2 := diffImage(mat2, diff, resolution/ratio)

			joined := joinImages(img1, img2, 5)

			filename := file1 + "-" + strconv.Itoa(page) + "-diff.png"
			file, err := os.Create(filename)
			if err != nil {
				return false, err
			}
			defer file.Close()

			pngJoined := rgbToPNG(joined)

			err = png.Encode(file, pngJoined)
			if err != nil {
				return false, fmt.Errorf("error writing %s to png: %w", file1, err)
			}
			if pdf != nil {
				pngFiles = append(pngFiles, PageFile{page, filename})
			}
		} else if !same {
			break
		}

	} // for all pages
	if pdf != nil && !same {
		err = BuildPDF(pngFiles, pdf)
		if err != nil {
			return false, err
		}
	}
	if same {
		return true, nil
	}
	return false, nil
}

func PageCount(filename string) (int, error) {

	rs, err := os.Open(filename)
	if err != nil {
		return 0, errors.New("pdfcpu: PDFInfo: missing rs")
	}

	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.LISTINFO

	ctx, err := api.ReadAndValidate(rs, conf)
	if err != nil {
		return 0, err
	}
	return ctx.PageCount, nil
}

func PdfToPPM(filename string, page, resolution int) (io.Reader, error) {
	pdftoppm := "pdftoppm"
	if runtime.GOOS == "windows" {
		pdftoppm = "pdftoppm.exe"
	}

	args := []string{
		"-r",
		strconv.Itoa(resolution),
		"-f",
		strconv.Itoa(page),
		"-l",
		strconv.Itoa(page),
		filename,
		"-",
	}
	cmd := exec.Command(pdftoppm, args...)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("pdftoppm start failed: %w, stderr: %s", err, stderrBuf.String())
	}

	// Wait for the command to finish
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("pdftoppm failed: %w, stderr: %s", err, stderrBuf.String())
	}

	return &stdoutBuf, nil
}

type PageFile struct {
	pageNum  int
	filename string
}

// Build a pdf file from a series of image files
func BuildPDF(imageFiles []PageFile, w io.Writer) error {

	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.CREATE
	//ctx, err := pdfcpu.CreateContextWithXRefTable(conf, types.PaperSize["A4L"])
	ctx, err := pdfcpu.CreateContextWithXRefTable(conf, types.PaperSize["A4"])
	if err != nil {
		return err
	}

	margin := 72.0
	pdf := &primitives.PDF{
		FieldIDs:      types.StringSet{},
		Fields:        types.Array{},
		FormFonts:     map[string]*primitives.FormFont{},
		Pages:         map[string]*primitives.PDFPage{},
		FontResIDs:    map[int]types.Dict{},
		XObjectResIDs: map[int]types.Dict{},
		Conf:          ctx.Configuration,
		XRefTable:     ctx.XRefTable,
		Optimize:      ctx.Optimize,
		CheckBoxAPs:   map[float64]*primitives.AP{},
		RadioBtnAPs:   map[float64]*primitives.AP{},
		OldFieldIDs:   types.StringSet{},
		Margins:       map[string]*primitives.Margin{},
		Paper:         "A4L",
		Origin:        "UpperLeft",
		Margin:        &primitives.Margin{Width: margin},
	}

	for _, pf := range imageFiles {
		thePage := primitives.PDFPage{}
		myImages := []*primitives.ImageBox{
			{Src: pf.filename, PageNr: strconv.Itoa(pf.pageNum), Position: [2]float64{0, 0}},
		}
		thePage.Content = &primitives.Content{
			ImageBoxes: myImages,
		}
		pdf.Pages[strconv.Itoa(pf.pageNum)] = &thePage
	}
	// Validate must come before RenderPages, since it adds the pages to the pdf
	if err := pdf.Validate(); err != nil {
		return err
	}

	pages, fontMap, err := pdf.RenderPages()
	if err != nil {
		return err
	}

	_, _, err = create.UpdatePageTree(ctx, pages, fontMap)
	if err != nil {
		return err
	}

	if conf.PostProcessValidate {
		if err = api.ValidateContext(ctx); err != nil {
			return err
		}
	}

	err = api.WriteContext(ctx, w)
	if err != nil {
		return (err)
	}

	return nil
}

# pdf-comp
### PDF File Visual Comparison Tool

This tool is mainly intended for testing environments where PDF files are generated programmatically, and should be exactly the same from one test run to the next.  It uses a bitmap comparison, so even the smallest differences in appearance will not be tolerated (some amount of fuzziness can be introduced by decreasing render resolution).  At the same time, the internal structure of the file and any non-visible features are not compared.

We can imagine some more interesting ways to compare PDFs, for example:

* Semantic or textual equivalence
* Indifference to image rotation / centering / brightness (useful for scanned images)
* Optical text recognition

But pdf-comp does none of those.  It just generates images for each page in the files using [pdftoppm](https://www.xpdfreader.com/pdftoppm-man.html) and checks if they are the same.  Optionally, it generates a side-by-side comparison image of differing pages, with the differences highlighted.

There is a command line program detailed below, but the real use of this package is to includimport it in an automated testing suite.

This code is inspired by SerHack's [pdf-diff](https://github.com/serhack/pdf-diff).  So thanks!  It leans heavily on Horst Rutter's amazing golang PDF processor, [pdfcpu](https://pdfcpu.io/). Also, Google's Gemini has made a significant contribution.

## Installation
It's important to use a recent version of XpdfReader - specifically *not* the Ubuntu package poppler-utils.  As of this writing, version 0.85 of pdftoppm included in poppler-utils is broken for output to stdout and will cause the program to fail.
Try using 
```
$ wget https://dl.xpdfreader.com/xpdf-tools-linux-4.05.tar.gz
$ tar -xzvf xpdf-tools-linux-4.05.targz
[move appropriate binaries into your PATH]
$ go install github.com/pdfcpu/pdfcpu/cmd/pdfcpu@latest
$ go install github.com/mdmcconnell/pdf-comp@latest
```

## API Usage
Have a look at cli.go for an example of how to use EqualPDFs
```
	same, err := EqualPDFs(file1, file2, images, w, resolution, ratio)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	} else if same {
		// do something
	}
	if same {
		// do something else
	}
```

## Command Line Operation
Usage: pdf-comp [options] file1.pdf file2.pdf 

### Options

**-images** if set, create images for each page that is different, highlighting the differences.  Names will be of the form file1.pdf-n-diff.png (with n being the page number)

**-pdf** compile page-by-page images into a single pdf file of differences

**-resolution=** *integer* dpi resolution for creating bitmaps, default 300dpi.  May impact performance.

**-radius=** *integer* radius of difference highlight circles to output, default 5pts.  Only meaninfgul if **images** is set

### Exit Codes
0    The two PDFs are visually equivalent
1    The two PDFs are not visually equivalent
2    The program has encountered some error before completing (and normally printed an error message)

## Example Comparison Output
This works especially well for fixed-width fonts (with variable-width, an entire line of text after the first different character will be highlighted).


![Lorem comparison][def]


[def]: https://github.com/mdmcconnell/pdf-comp/blob/f8abf71bbb09771c9b248301c2e9822cee40a101/assets/loremM.pdf-1-diff.png "Differences highlighted"
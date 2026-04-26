package pdf

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/jcroyoaun/totalcompmx/internal/database"
	"github.com/jcroyoaun/totalcompmx/internal/pdfreport"
)

type OtherBenefit = pdfreport.OtherBenefit
type PackageInput = pdfreport.PackageInput
type PackageResult = pdfreport.PackageResult
type ReportData = pdfreport.ReportData

var renderComparisonHTML = pdfreport.RenderComparisonHTML
var renderPDF = renderHTMLToPDF
var newPDFRenderer = newChromePDFRenderer

// GenerateComparisonReport generates a PDF by rendering an HTML template via Chrome
func GenerateComparisonReport(packages []PackageResult, fiscalYear database.FiscalYear) ([]byte, error) {
	htmlContent, err := renderComparisonHTML(packages)
	if err != nil {
		return nil, err
	}

	pdfBytes, err := renderPDF(htmlContent)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML to PDF: %w", err)
	}

	return pdfBytes, nil
}

// renderHTMLToPDF uses chromedp to render HTML to PDF
func renderHTMLToPDF(htmlContent string) ([]byte, error) {
	return newPDFRenderer().render(htmlContent)
}

type chromePDFRenderer struct {
	newExecAllocator  func(context.Context, ...chromedp.ExecAllocatorOption) (context.Context, context.CancelFunc)
	newContext        func(context.Context, ...chromedp.ContextOption) (context.Context, context.CancelFunc)
	withTimeout       func(context.Context, time.Duration) (context.Context, context.CancelFunc)
	navigate          func(string) chromedp.Action
	run               func(context.Context, ...chromedp.Action) error
	frameTree         func(context.Context) (*page.FrameTree, error)
	setDocument       func(context.Context, cdp.FrameID, string) error
	printToPDF        func(context.Context) ([]byte, error)
	sleep             func(time.Duration)
	renderingDeadline time.Duration
}

func newChromePDFRenderer() chromePDFRenderer {
	return chromePDFRenderer{
		newExecAllocator: chromedp.NewExecAllocator,
		newContext:       chromedp.NewContext,
		withTimeout:      context.WithTimeout,
		navigate: func(location string) chromedp.Action {
			return chromedp.Navigate(location)
		},
		run: chromedp.Run,
		frameTree: func(ctx context.Context) (*page.FrameTree, error) {
			return page.GetFrameTree().Do(ctx)
		},
		setDocument: func(ctx context.Context, frameID cdp.FrameID, htmlContent string) error {
			return page.SetDocumentContent(frameID, htmlContent).Do(ctx)
		},
		printToPDF: func(ctx context.Context) ([]byte, error) {
			pdfBuf, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				WithScale(0.75).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				Do(ctx)
			return pdfBuf, err
		},
		sleep:             time.Sleep,
		renderingDeadline: 30 * time.Second,
	}
}

func (r chromePDFRenderer) render(htmlContent string) ([]byte, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
	)

	allocCtx, allocCancel := r.newExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := r.newContext(allocCtx)
	defer cancel()

	ctx, cancel = r.withTimeout(ctx, r.renderingDeadline)
	defer cancel()

	return r.printHTML(ctx, htmlContent)
}

func (r chromePDFRenderer) printHTML(ctx context.Context, htmlContent string) ([]byte, error) {
	var pdfBuf []byte
	err := r.run(ctx,
		r.navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return r.setHTML(ctx, htmlContent)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, err = r.print(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("chromedp error: %w", err)
	}

	return pdfBuf, nil
}

func (r chromePDFRenderer) setHTML(ctx context.Context, htmlContent string) error {
	frameTree, err := r.frameTree(ctx)
	if err != nil {
		return err
	}

	return r.setDocument(ctx, frameTree.Frame.ID, htmlContent)
}

func (r chromePDFRenderer) print(ctx context.Context) ([]byte, error) {
	r.sleep(500 * time.Millisecond)
	return r.printToPDF(ctx)
}

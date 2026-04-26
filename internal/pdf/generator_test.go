package pdf

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/jcroyoaun/totalcompmx/internal/database"
)

func TestGenerateComparisonReport(t *testing.T) {
	restoreHTML := stubRenderComparisonHTML(func(packages []PackageResult) (string, error) {
		return "<html>ok</html>", nil
	})
	defer restoreHTML()
	restorePDF := stubRenderPDF(func(htmlContent string) ([]byte, error) {
		return []byte("%PDF"), nil
	})
	defer restorePDF()

	pdfBytes, err := GenerateComparisonReport(nil, database.FiscalYear{})

	if err != nil {
		t.Fatal(err)
	}
	if string(pdfBytes) != "%PDF" {
		t.Fatalf("got %q; want PDF bytes", pdfBytes)
	}
}

func TestGenerateComparisonReportErrors(t *testing.T) {
	t.Run("HTML render error", func(t *testing.T) {
		restoreHTML := stubRenderComparisonHTML(func(packages []PackageResult) (string, error) {
			return "", errors.New("html failed")
		})
		defer restoreHTML()

		_, err := GenerateComparisonReport(nil, database.FiscalYear{})

		assertErrorContains(t, err, "html failed")
	})

	t.Run("PDF render error", func(t *testing.T) {
		restoreHTML := stubRenderComparisonHTML(func(packages []PackageResult) (string, error) {
			return "<html>ok</html>", nil
		})
		defer restoreHTML()
		restorePDF := stubRenderPDF(func(htmlContent string) ([]byte, error) {
			return nil, errors.New("pdf failed")
		})
		defer restorePDF()

		_, err := GenerateComparisonReport(nil, database.FiscalYear{})

		assertErrorContains(t, err, "failed to render HTML to PDF")
	})
}

func TestRenderHTMLToPDF(t *testing.T) {
	renderer := fakePDFRenderer(t)
	restore := stubPDFRenderer(renderer)
	defer restore()

	pdfBytes, err := renderHTMLToPDF("<html>ok</html>")

	if err != nil {
		t.Fatal(err)
	}
	if string(pdfBytes) != "%PDF" {
		t.Fatalf("got %q; want fake PDF bytes", pdfBytes)
	}
}

func TestChromePDFRendererErrors(t *testing.T) {
	t.Run("run error", func(t *testing.T) {
		renderer := fakePDFRenderer(t)
		renderer.run = func(context.Context, ...chromedp.Action) error {
			return errors.New("run failed")
		}

		_, err := renderer.render("<html>ok</html>")

		assertErrorContains(t, err, "chromedp error")
	})

	t.Run("frame tree error", func(t *testing.T) {
		renderer := fakePDFRenderer(t)
		renderer.frameTree = func(context.Context) (*page.FrameTree, error) {
			return nil, errors.New("frame failed")
		}

		_, err := renderer.render("<html>ok</html>")

		assertErrorContains(t, err, "frame failed")
	})

	t.Run("print error", func(t *testing.T) {
		renderer := fakePDFRenderer(t)
		renderer.printToPDF = func(context.Context) ([]byte, error) {
			return nil, errors.New("print failed")
		}

		_, err := renderer.render("<html>ok</html>")

		assertErrorContains(t, err, "print failed")
	})
}

func TestNewChromePDFRenderer(t *testing.T) {
	renderer := newChromePDFRenderer()
	ctx := cdp.WithExecutor(context.Background(), fakeCDPExecutor{})

	if renderer.renderingDeadline != 30*time.Second {
		t.Fatalf("got %s; want 30s", renderer.renderingDeadline)
	}
	_ = renderer.navigate("about:blank")
	if _, err := renderer.frameTree(ctx); err != nil {
		t.Fatal(err)
	}
	if err := renderer.setDocument(ctx, "root", "<html>ok</html>"); err != nil {
		t.Fatal(err)
	}
	pdfBytes, err := renderer.printToPDF(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(pdfBytes) != "%PDF" {
		t.Fatalf("got %q; want fake PDF bytes", pdfBytes)
	}
}

func fakePDFRenderer(t *testing.T) chromePDFRenderer {
	return chromePDFRenderer{
		newExecAllocator: func(ctx context.Context, opts ...chromedp.ExecAllocatorOption) (context.Context, context.CancelFunc) {
			return ctx, func() {}
		},
		newContext: func(ctx context.Context, opts ...chromedp.ContextOption) (context.Context, context.CancelFunc) {
			return ctx, func() {}
		},
		withTimeout: func(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
			return ctx, func() {}
		},
		navigate: func(location string) chromedp.Action {
			return chromedp.ActionFunc(func(context.Context) error {
				return nil
			})
		},
		run: func(ctx context.Context, actions ...chromedp.Action) error {
			for _, action := range actions {
				if err := action.Do(ctx); err != nil {
					return err
				}
			}
			return nil
		},
		frameTree: func(context.Context) (*page.FrameTree, error) {
			return &page.FrameTree{Frame: &cdp.Frame{ID: "root"}}, nil
		},
		setDocument: func(ctx context.Context, frameID cdp.FrameID, htmlContent string) error {
			if frameID == "" || htmlContent == "" {
				t.Fatal("missing frame ID or HTML content")
			}
			return nil
		},
		printToPDF: func(context.Context) ([]byte, error) {
			return []byte("%PDF"), nil
		},
		sleep:             func(time.Duration) {},
		renderingDeadline: time.Second,
	}
}

func stubRenderComparisonHTML(fn func([]PackageResult) (string, error)) func() {
	original := renderComparisonHTML
	renderComparisonHTML = fn
	return func() {
		renderComparisonHTML = original
	}
}

func stubRenderPDF(fn func(string) ([]byte, error)) func() {
	original := renderPDF
	renderPDF = fn
	return func() {
		renderPDF = original
	}
}

func stubPDFRenderer(renderer chromePDFRenderer) func() {
	original := newPDFRenderer
	newPDFRenderer = func() chromePDFRenderer {
		return renderer
	}
	return func() {
		newPDFRenderer = original
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q; got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("got error %q; want to contain %q", err, want)
	}
}

type fakeCDPExecutor struct{}

func (fakeCDPExecutor) Execute(ctx context.Context, method string, params any, res any) error {
	switch method {
	case page.CommandGetFrameTree:
		res.(*page.GetFrameTreeReturns).FrameTree = &page.FrameTree{Frame: &cdp.Frame{ID: "root"}}
	case page.CommandPrintToPDF:
		res.(*page.PrintToPDFReturns).Data = base64.StdEncoding.EncodeToString([]byte("%PDF"))
	}
	return nil
}

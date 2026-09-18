package checker

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/sentinel-monitoring/sentinel/internal/models"
)

const nginx503 = `<html>
<head><title>503 Service Temporarily Unavailable</title></head>
<body>
<center><h1>503 Service Temporarily Unavailable</h1></center>
<hr><center>nginx</center>
</body>
</html>`

const nginx502 = `<html>
<head><title>502 Bad Gateway</title></head>
<body>
<center><h1>502 Bad Gateway</h1></center>
<hr><center>nginx</center>
</body>
</html>`

const shopwareMaintenance = `<!DOCTYPE html>
<html>
<head><title>Maintenance</title></head>
<body class="is--maintenance">
<div class="sw-maintenance">Our shop is currently undergoing maintenance.</div>
</body>
</html>`

func TestClassifyHTTPErrorShopwareVsNginxVsPHPFPM(t *testing.T) {
	shopwareHdr := http.Header{"Server": []string{"nginx"}}
	if got := classifyHTTPError(503, shopwareHdr, shopwareMaintenance); got != models.HTTPErrorSourceShopware {
		t.Fatalf("shopware behind nginx: got %s", got)
	}

	nginxHdr := http.Header{"Server": []string{"nginx"}}
	if got := classifyHTTPError(503, nginxHdr, nginx503); got != models.HTTPErrorSourceNginx {
		t.Fatalf("nginx 503: got %s", got)
	}

	if got := classifyHTTPError(502, nginxHdr, nginx502); got != models.HTTPErrorSourcePHPFPM {
		t.Fatalf("nginx 502: got %s want php_fpm", got)
	}

	if got := classifyHTTPError(500, http.Header{"X-Powered-By": []string{"PHP/8.3"}}, "Primary script unknown"); got != models.HTTPErrorSourcePHPFPM {
		t.Fatalf("php-fpm script: got %s", got)
	}

	cf := http.Header{"CF-Ray": []string{"abc"}, "Server": []string{"cloudflare"}}
	if got := classifyHTTPError(503, cf, "error"); got != models.HTTPErrorSourceCloudflare {
		t.Fatalf("cloudflare: got %s", got)
	}
}

func TestAttachHTTPErrorPageCapturesBodyAndCompactDetails(t *testing.T) {
	code := 503
	result := &models.CheckResult{Status: models.StatusDown, StatusCode: &code}
	hdr := http.Header{}
	hdr.Set("Server", "nginx")
	hdr.Set("Content-Type", "text/html")
	attachHTTPErrorPage(result, "https://shop.example/maintenance", hdr, shopwareMaintenance)
	if result.ErrorPage == nil {
		t.Fatal("expected error page")
	}
	if result.ErrorPage.Source != models.HTTPErrorSourceShopware {
		t.Fatalf("source=%s", result.ErrorPage.Source)
	}
	if !strings.Contains(result.ErrorPage.BodyHTML, "sw-maintenance") {
		t.Fatalf("body=%q", result.ErrorPage.BodyHTML)
	}
	if result.ErrorPage.Headers["Server"] != "nginx" {
		t.Fatalf("headers=%v", result.ErrorPage.Headers)
	}

	var compact models.HTTPErrorPage
	if err := json.Unmarshal([]byte(result.Details), &compact); err != nil {
		t.Fatal(err)
	}
	if compact.BodyHTML != "" {
		t.Fatal("compact details must omit HTML body")
	}
	if compact.Source != models.HTTPErrorSourceShopware {
		t.Fatalf("compact source=%s", compact.Source)
	}
	if compact.ViewToken != "" {
		t.Fatal("compact details must omit view token")
	}
}

func TestAttachHTTPErrorPageNginx503(t *testing.T) {
	code := 503
	result := &models.CheckResult{Status: models.StatusDown, StatusCode: &code}
	hdr := http.Header{"Server": []string{"nginx"}}
	attachHTTPErrorPage(result, "https://shop.example/", hdr, nginx503)
	if result.ErrorPage.Source != models.HTTPErrorSourceNginx {
		t.Fatalf("source=%s", result.ErrorPage.Source)
	}
}

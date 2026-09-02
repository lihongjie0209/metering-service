package httptransport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lihongjie0209/metering-service/internal/metering"
)

func TestMeterViewDoesNotExposeStoredDimensionJSON(t *testing.T) {
	t.Parallel()
	const internalJSON = `{"internal":"secret-value"}`
	encoded, err := json.Marshal(meterView(metering.Meter{ID: "meter-1", DimensionKeysJSON: internalJSON}))
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	if strings.Contains(body, "dimension_keys_json") || strings.Contains(body, "secret-value") {
		t.Fatalf("meter response exposed storage representation: %s", body)
	}
}

func TestUsagePageResponseMapsTotals(t *testing.T) {
	t.Parallel()
	page := metering.Page[metering.UsagePoint]{Items: []metering.UsagePoint{{Quantity: 7}}, Total: 3, Page: 2, PageSize: 1}
	response := usagePageResponse(page, 21)
	if len(response.Items) != 1 || response.Items[0].Quantity != 7 || response.Total != 3 || response.Page != 2 || response.PageSize != 1 || response.TotalQuantity != 21 {
		t.Fatalf("usagePageResponse() = %+v", response)
	}
}

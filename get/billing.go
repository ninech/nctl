package get

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ninech/nctl/api"
	"sigs.k8s.io/yaml"
)

type billingCmd struct {
	BillingAPIURL string `help:"Base URL of the billing API." default:"http://localhost:8080" name:"billing-api-url"`
}

type subscriptionsResponse struct {
	CustomerIdentifier string         `json:"customer_identifier"`
	Subscriptions      []subscription `json:"subscriptions"`
	Total              int            `json:"total"`
}

type subscription struct {
	SubscriptionID      int                 `json:"subscription_id"`
	SubscriptionCode    string              `json:"subscription_code"`
	ResourceName        string              `json:"resource_name"`
	CustomerDescription string              `json:"customer_description"`
	URN                 string              `json:"urn"`
	ValidFrom           string              `json:"valid_from"`
	ValidTo             *string             `json:"valid_to"`
	Product             subscriptionProduct `json:"product"`
	Options             json.RawMessage     `json:"options"`
}

type subscriptionProduct struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	PriceUnit float64 `json:"price_unit"`
}

func (cmd *billingCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	org, err := client.Organization()
	if err != nil {
		return fmt.Errorf("unable to get organization: %w", err)
	}

	token := client.Token(ctx)
	if token == "" {
		return fmt.Errorf("unable to get authentication token")
	}

	reqURL := fmt.Sprintf("%s/api/cockpit/subscriptions?customer_identifier=%s", cmd.BillingAPIURL, url.QueryEscape(org))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to call billing API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("billing API returned status %d: %s", resp.StatusCode, string(body))
	}

	switch get.Format {
	case jsonOut:
		var indented bytes.Buffer
		if err := json.Indent(&indented, body, "", "  "); err != nil {
			return fmt.Errorf("unable to format response: %w", err)
		}
		get.Printf("%s\n", indented.String())
	case yamlOut:
		var raw interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return fmt.Errorf("unable to parse response: %w", err)
		}
		yamlBytes, err := yaml.Marshal(raw)
		if err != nil {
			return fmt.Errorf("unable to convert to YAML: %w", err)
		}
		get.Printf("%s", string(yamlBytes))
	default:
		var result subscriptionsResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("unable to parse response: %w", err)
		}

		if len(result.Subscriptions) == 0 {
			get.Printf("no subscriptions found for organization %q\n", org)
			return nil
		}

		if get.Format != noHeader {
			get.writeTabRow("", "SUBSCRIPTION", "RESOURCE", "PRODUCT", "QTY", "PRICE")
		}
		for _, s := range result.Subscriptions {
			get.writeTabRow("",
				s.SubscriptionCode,
				s.ResourceName,
				s.Product.ProductID,
				fmt.Sprintf("%.1f", s.Product.Quantity),
				fmt.Sprintf("%.2f", s.Product.PriceUnit),
			)
		}
		return get.tabWriter.Flush()
	}

	return nil
}

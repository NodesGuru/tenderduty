package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	defaultCoinmarketcapApiEndpoint = "https://pro-api.coinmarketcap.com"
	defaultRequestTimeout           = 10 * time.Second
)

// CryptoPrice represents price data for a cryptocurrency
type CryptoPrice struct {
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Symbol      string    `json:"symbol"`
	Currency    string    `json:"currency"`
	Price       float64   `json:"price"`
	LastUpdated time.Time `json:"last_updated"`
}

// CMCResponse represents the structure of the CoinMarketCap API response
type CMCResponse struct {
	Status struct {
		Timestamp    string `json:"timestamp"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
	} `json:"status"`
	Data map[string]struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
		Slug   string `json:"slug"`
		Quote  map[string]struct {
			Price       float64 `json:"price"`
			LastUpdated string  `json:"last_updated"`
		} `json:"quote"`
	} `json:"data"`
}

// CoinMarketCapClient handles API requests to CoinMarketCap
type CoinMarketCapClient struct {
	apiKey          string
	currency        string
	cacheExpiration int
	slugs           []string
	apiEndpoint     string
	httpClient      *http.Client
	cache           struct {
		data       map[string]CryptoPrice
		lastUpdate time.Time
		mu         sync.RWMutex
	}
}

// NewCoinMarketCapClient creates a new client with the provided API key
func NewCoinMarketCapClient(apiKey string, currency string, cacheExpiration int, slugs []string) *CoinMarketCapClient {
	client := &CoinMarketCapClient{
		apiKey:          apiKey,
		currency:        currency,
		cacheExpiration: cacheExpiration,
		slugs:           slugs,
		apiEndpoint:     defaultCoinmarketcapApiEndpoint,
		httpClient: &http.Client{
			Timeout: defaultRequestTimeout,
		},
	}

	client.cache.data = make(map[string]CryptoPrice)

	return client
}

// WithEndpoint allows customizing the API endpoint URL
func WithEndpoint(endpoint string) func(*CoinMarketCapClient) {
	return func(c *CoinMarketCapClient) {
		c.apiEndpoint = endpoint
	}
}

// WithTimeout allows customizing the HTTP client timeout
func WithTimeout(timeout time.Duration) func(*CoinMarketCapClient) {
	return func(c *CoinMarketCapClient) {
		c.httpClient.Timeout = timeout
	}
}

// getCachedData retrieves data from the cache if it's valid and not expired
func (c *CoinMarketCapClient) getCachedData() (map[string]CryptoPrice, bool, time.Time) {
	c.cache.mu.RLock()
	defer c.cache.mu.RUnlock()

	cacheAge := time.Since(c.cache.lastUpdate)
	hasCache := !c.cache.lastUpdate.IsZero()
	lastUpdate := c.cache.lastUpdate

	// If cache is valid and not expired, return cached data
	if hasCache && cacheAge < time.Duration(c.cacheExpiration) {
		result := make(map[string]CryptoPrice, len(c.cache.data))
		for k, v := range c.cache.data {
			result[k] = v
		}
		return result, true, lastUpdate
	}

	return nil, hasCache, lastUpdate
}

// updateCache updates the cache with new data
func (c *CoinMarketCapClient) updateCache(prices map[string]CryptoPrice) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()

	c.cache.data = prices
	c.cache.lastUpdate = time.Now()
}

// GetPrices fetches cryptocurrency prices, using cache when available
func (c *CoinMarketCapClient) GetPrices(ctx context.Context) (map[string]CryptoPrice, error) {
	// Try to get from cache first
	cachedData, cacheValid, _ := c.getCachedData()
	if cacheValid {
		return cachedData, nil
	}

	// Cache expired or doesn't exist, fetch fresh data
	prices, err := c.fetchPricesFromAPI(ctx, c.slugs, c.currency)
	if err != nil {
		return nil, err
	}

	// Update cache
	c.updateCache(prices)
	return prices, nil
}

// GetPrice fetches the price for a specific cryptocurrency slug, using cache when available
func (c *CoinMarketCapClient) GetPrice(ctx context.Context, slug string) (*CryptoPrice, error) {
	// Try to get from cache first
	cachedData, cacheValid, _ := c.getCachedData()
	if cacheValid {
		if price, exists := cachedData[slug]; exists {
			return &price, nil
		}
	}

	// Cache expired, doesn't exist, or slug not found - fetch fresh data
	prices, err := c.fetchPricesFromAPI(ctx, c.slugs, c.currency)
	if err != nil {
		return nil, err
	}

	// Update cache
	c.updateCache(prices)

	// Check if the slug exists in the freshly fetched data
	if price, exists := prices[slug]; exists {
		return &price, nil
	}

	// Slug not found even after refreshing the data
	return nil, fmt.Errorf("slug '%s' not found", slug)
}

// fetchPricesFromAPI makes the actual API call to CoinMarketCap
func (c *CoinMarketCapClient) fetchPricesFromAPI(ctx context.Context, slugs []string, currency string) (map[string]CryptoPrice, error) {
	url := c.apiEndpoint + "/v2/cryptocurrency/quotes/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add required headers
	req.Header.Add("X-CMC_PRO_API_KEY", c.apiKey)
	req.Header.Add("Accept", "application/json")

	// Add query parameters
	q := req.URL.Query()
	if len(slugs) > 0 {
		q.Add("slug", joinStrings(slugs, ","))
	}
	q.Add("convert", currency)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var cmcResp CMCResponse
	if err := json.NewDecoder(resp.Body).Decode(&cmcResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Check for API error
	if cmcResp.Status.ErrorCode != 0 {
		return nil, fmt.Errorf("API returned error: %s", cmcResp.Status.ErrorMessage)
	}

	// Extract the prices
	result := make(map[string]CryptoPrice, len(cmcResp.Data))
	for _, cryptoData := range cmcResp.Data {
		quoteData, ok := cryptoData.Quote[currency]
		if !ok {
			continue // Skip if the requested currency quote isn't available
		}

		lastUpdated, err := time.Parse("2006-01-02T15:04:05.000Z", quoteData.LastUpdated)
		if err != nil {
			// If time parsing fails, use current time as fallback
			lastUpdated = time.Now()
		}

		result[cryptoData.Slug] = CryptoPrice{
			Name:        cryptoData.Name,
			Symbol:      cryptoData.Symbol,
			Slug:        cryptoData.Slug,
			Currency:    currency,
			Price:       quoteData.Price,
			LastUpdated: lastUpdated,
		}
	}

	return result, nil
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}

	return result
}

// ConvertDecCoinToDisplayUnit converts a DecCoin array to the display unit based on DenomMetadata.
func ConvertDecCoinToDisplayUnit(coins []github_com_cosmos_cosmos_sdk_types.DecCoin, metadata bank.Metadata) (*github_com_cosmos_cosmos_sdk_types.DecCoins, error) {
	var convertedCoins github_com_cosmos_cosmos_sdk_types.DecCoins

	// Find the display denomination unit
	var displayDenom string
	var displayExponent uint32

	// If no display is set, default to base
	if metadata.Display == "" {
		displayDenom = metadata.Base
	} else {
		displayDenom = metadata.Display
	}

	// Find the exponent for the display denom
	foundDisplayDenom := false
	for _, unit := range metadata.DenomUnits {
		if unit.Denom == displayDenom {
			displayExponent = unit.Exponent
			foundDisplayDenom = true
			break
		}
	}

	if !foundDisplayDenom {
		return nil, fmt.Errorf("display unit '%s' not found in denomination units for: %s", displayDenom, metadata.Base)
	}

	for _, coin := range coins {
		// If the coin is already in the display denomination, just add it
		if coin.Denom == displayDenom {
			convertedCoins = append(convertedCoins, coin)
			continue
		}

		// Find current coin's exponent to properly calculate conversion
		var currentExponent uint32 = 0
		foundCurrentDenom := false
		for _, unit := range metadata.DenomUnits {
			if unit.Denom == coin.Denom {
				currentExponent = unit.Exponent
				foundCurrentDenom = true
				break
			}
		}

		if !foundCurrentDenom {
			return nil, fmt.Errorf("source denomination '%s' not found in denomination units", coin.Denom)
		}

		// Calculate the conversion factor based on exponent difference
		var convertedAmount github_com_cosmos_cosmos_sdk_types.Dec

		if currentExponent < displayExponent {
			// Converting from smaller to larger unit (e.g., uatom -> atom)
			// We need to divide by 10^(display_exp - current_exp)
			exponentDiff := displayExponent - currentExponent

			// Create a decimal with the proper power of 10
			divisor := github_com_cosmos_cosmos_sdk_types.OneDec()
			for i := uint32(0); i < exponentDiff; i++ {
				divisor = divisor.MulInt64(10)
			}

			convertedAmount = coin.Amount.Quo(divisor)
		} else {
			// Converting from larger to smaller unit (e.g., atom -> uatom)
			// We need to multiply by 10^(current_exp - display_exp)
			exponentDiff := currentExponent - displayExponent

			// Create a decimal with the proper power of 10
			multiplier := github_com_cosmos_cosmos_sdk_types.OneDec()
			for i := uint32(0); i < exponentDiff; i++ {
				multiplier = multiplier.MulInt64(10)
			}

			convertedAmount = coin.Amount.Mul(multiplier)
		}

		convertedCoins = append(convertedCoins, github_com_cosmos_cosmos_sdk_types.NewDecCoinFromDec(displayDenom, convertedAmount))
	}

	return &convertedCoins, nil
}

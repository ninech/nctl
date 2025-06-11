package auth

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/int128/kubelogin/pkg/tokencache"
	"github.com/int128/kubelogin/pkg/tokencache/repository"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/client-go/util/homedir"
)

type LogoutCmd struct {
	APIURL    string `help:"The URL of the Nine API" default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL string `help:"Issuer URL is the OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID  string `help:"Client ID is the OIDC client ID of the API." default:"nineapis.ch-f178254"`
}

func (l *LogoutCmd) Run(ctx context.Context, command string, tk api.TokenGetter) error {
	key := tokencache.Key{
		ClientID:  l.ClientID,
		IssuerURL: l.IssuerURL,
	}

	filename, err := computeFilename(key)
	if err != nil {
		return err
	}
	filePath := path.Join(homedir.HomeDir(), api.DefaultTokenCachePath, filename)

	if _, err = os.Stat(filePath); err != nil {
		format.PrintFailuref("🤔", "seems like you are already logged out from %s", l.APIURL)
		return nil
	}

	r := repository.Repository{}
	cache, err := r.FindByKey(path.Join(homedir.HomeDir(), api.DefaultTokenCachePath), key)
	if err != nil {
		return fmt.Errorf("error finding cache file: %w", err)
	}

	form := url.Values{}
	form.Add("client_id", l.ClientID)
	form.Add("refresh_token", cache.RefreshToken)

	logoutEndpoint := strings.Join([]string{l.IssuerURL, "protocol", "openid-connect", "logout"}, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, logoutEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	token, err := tk.GetTokenString(ctx, l.IssuerURL, l.ClientID, false)
	if err != nil {
		return fmt.Errorf("error getting token: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)

	c := http.DefaultClient
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("error logging out from OIDC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode <= 200 && resp.StatusCode > 300 {
		return fmt.Errorf("http request error %d to %s", resp.StatusCode, logoutEndpoint)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error removing the local cache: %w", err)
	}

	format.PrintSuccessf("👋", "logged out from %s", l.APIURL)

	return nil
}

// This function is copied from kubelogin to find out the cache filename
// See: github.com/int128/kubelogin/pkg/tokencache/repository
func computeFilename(key tokencache.Key) (string, error) {
	s := sha256.New()
	e := gob.NewEncoder(s)
	if err := e.Encode(&key); err != nil {
		return "", fmt.Errorf("could not encode the key: %w", err)
	}
	h := hex.EncodeToString(s.Sum(nil))
	return h, nil
}

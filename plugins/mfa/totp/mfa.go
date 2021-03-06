package totp

import (
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/Luzifer/nginx-sso/plugins"
)

type MFATOTP struct{}

// ProviderID needs to return an unique string to identify
// this special MFA provider
func (m MFATOTP) ProviderID() (id string) {
	return "totp"
}

func New() *MFATOTP {
	return &MFATOTP{}
}

// Configure loads the configuration for the Authenticator from the
// global config.yaml file which is passed as a byte-slice.
// If no configuration for the Authenticator is supplied the function
// needs to return the plugins.ErrProviderUnconfigured
func (m MFATOTP) Configure(yamlSource []byte) (err error) { return nil }

// ValidateMFA takes the user from the login cookie and performs a
// validation against the provided MFA configuration for this user
func (m MFATOTP) ValidateMFA(res http.ResponseWriter, r *http.Request, user string, mfaCfgs []plugins.MFAConfig) error {
	// Look for mfaConfigs with own provider name
	for _, c := range mfaCfgs {
		// Provider has been renamed, keep "google" for backwards compatibility
		if c.Provider != m.ProviderID() && c.Provider != "google" {
			continue
		}

		token, err := m.exec(c)
		if err != nil {
			return errors.Wrap(err, "Generating the MFA token failed")
		}

		for key, values := range r.Form {
			if strings.HasSuffix(key, plugins.MFALoginFieldName) && values[0] == token {
				return nil
			}
		}
	}

	// Report this provider was not able to verify the MFA request
	return plugins.ErrNoValidUserFound
}

func (m MFATOTP) exec(c plugins.MFAConfig) (string, error) {
	secret := c.AttributeString("secret")

	// By default use Google Authenticator compatible settings
	generatorOpts := totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}

	if period := c.AttributeInt("period"); period > 0 {
		generatorOpts.Period = uint(period)
	}

	if skew := c.AttributeInt("skew"); skew > 0 {
		generatorOpts.Skew = uint(skew)
	}

	if digits := c.AttributeInt("digits"); digits > 0 {
		generatorOpts.Digits = otp.Digits(digits)
	}

	if algorithm := c.AttributeString("algorithm"); algorithm != "" {
		switch algorithm {
		case "sha1":
			generatorOpts.Algorithm = otp.AlgorithmSHA1
		case "sha256":
			generatorOpts.Algorithm = otp.AlgorithmSHA256
		case "sha512":
			generatorOpts.Algorithm = otp.AlgorithmSHA512
		default:
			return "", errors.Errorf("Unsupported algorithm %q", algorithm)
		}
	}

	if n := len(secret) % 8; n != 0 {
		secret += strings.Repeat("=", 8-n)
	}

	return totp.GenerateCodeCustom(strings.ToUpper(secret), time.Now(), generatorOpts)
}

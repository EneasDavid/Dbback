package httpapi

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"feedback/pkg/app"

	"github.com/fxamacker/cbor/v2"
)

type PasskeyController struct{}

const (
	passkeyCookieName          = "feedback_passkey"
	passkeyChallengeCookieName = "feedback_passkey_challenge"
	passkeyChallengeTTL        = 5 * time.Minute
	passkeyCookieTTL           = 180 * 24 * time.Hour
	webauthnCreateType         = "webauthn.create"
	webauthnGetType            = "webauthn.get"
)

type passkeyChallenge struct {
	Kind      string `json:"kind"`
	Challenge string `json:"challenge"`
	Matricula string `json:"matricula,omitempty"`
	UserID    string `json:"userId,omitempty"`
	Expires   int64  `json:"expires"`
}

type storedPasskey struct {
	Version      int              `json:"version"`
	User         app.SessionUser  `json:"user"`
	UserID       string           `json:"userId"`
	CredentialID string           `json:"credentialId"`
	PublicKey    passkeyPublicKey `json:"publicKey"`
	SignCount    uint32           `json:"signCount"`
	CreatedAt    int64            `json:"createdAt"`
	UpdatedAt    int64            `json:"updatedAt"`
}

type passkeyPublicKey struct {
	Alg int    `json:"alg"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type passkeyOptionsResponse struct {
	PublicKey any `json:"publicKey"`
}

type passkeyCreationOptions struct {
	Challenge              string                        `json:"challenge"`
	RP                     passkeyRPEntity               `json:"rp"`
	User                   passkeyUserEntity             `json:"user"`
	PubKeyCredParams       []passkeyCredentialParam      `json:"pubKeyCredParams"`
	Timeout                int                           `json:"timeout"`
	Attestation            string                        `json:"attestation"`
	AuthenticatorSelection passkeyAuthenticatorSelection `json:"authenticatorSelection"`
	ExcludeCredentials     []passkeyCredentialDescriptor `json:"excludeCredentials,omitempty"`
}

type passkeyRequestOptions struct {
	Challenge        string                        `json:"challenge"`
	RPID             string                        `json:"rpId"`
	AllowCredentials []passkeyCredentialDescriptor `json:"allowCredentials,omitempty"`
	UserVerification string                        `json:"userVerification"`
	Timeout          int                           `json:"timeout"`
}

type passkeyRPEntity struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type passkeyUserEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type passkeyCredentialParam struct {
	Type string `json:"type"`
	Alg  int    `json:"alg"`
}

type passkeyCredentialDescriptor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type passkeyAuthenticatorSelection struct {
	ResidentKey        string `json:"residentKey"`
	RequireResidentKey bool   `json:"requireResidentKey"`
	UserVerification   string `json:"userVerification"`
}

type attestationRequest struct {
	ID       string `json:"id"`
	RawID    string `json:"rawId"`
	Type     string `json:"type"`
	Response struct {
		AttestationObject string `json:"attestationObject"`
		ClientDataJSON    string `json:"clientDataJSON"`
	} `json:"response"`
}

type assertionRequest struct {
	ID       string `json:"id"`
	RawID    string `json:"rawId"`
	Type     string `json:"type"`
	Response struct {
		AuthenticatorData string `json:"authenticatorData"`
		ClientDataJSON    string `json:"clientDataJSON"`
		Signature         string `json:"signature"`
		UserHandle        string `json:"userHandle,omitempty"`
	} `json:"response"`
}

type webauthnClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

type attestationObject struct {
	AuthData []byte `cbor:"authData"`
}

type parsedAuthData struct {
	RPIDHash     []byte
	Flags        byte
	SignCount    uint32
	CredentialID []byte
	PublicKey    passkeyPublicKey
}

func (PasskeyController) RegisterOptions(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg, sessions, ok := passkeyRuntime(w)
	if !ok {
		return
	}
	user, ok := sessions.User(r)
	if !ok {
		app.Error(w, app.NewHTTPError(http.StatusUnauthorized, "entre antes de salvar a chave de acesso"))
		return
	}
	challenge, err := randomBase64URL(32)
	if err != nil {
		app.Error(w, err)
		return
	}
	userID, err := randomBase64URL(32)
	if err != nil {
		app.Error(w, err)
		return
	}
	stored, _ := readPasskeyCookie(r, cfg)
	setPasskeyChallenge(w, cfg, passkeyChallenge{
		Kind:      "register",
		Challenge: challenge,
		Matricula: user.Matricula,
		UserID:    userID,
		Expires:   time.Now().Add(passkeyChallengeTTL).Unix(),
	})
	options := passkeyCreationOptions{
		Challenge: challenge,
		RP: passkeyRPEntity{
			Name: "dbBack",
			ID:   passkeyRPID(r),
		},
		User: passkeyUserEntity{
			ID:          userID,
			Name:        user.Matricula,
			DisplayName: firstNonEmpty(user.Name, user.Matricula),
		},
		PubKeyCredParams: []passkeyCredentialParam{
			{Type: "public-key", Alg: -7},
		},
		Timeout:     60_000,
		Attestation: "none",
		AuthenticatorSelection: passkeyAuthenticatorSelection{
			ResidentKey:        "preferred",
			RequireResidentKey: false,
			UserVerification:   "preferred",
		},
	}
	if stored.CredentialID != "" {
		options.ExcludeCredentials = []passkeyCredentialDescriptor{{Type: "public-key", ID: stored.CredentialID}}
	}
	app.JSON(w, http.StatusOK, passkeyOptionsResponse{PublicKey: options})
}

func (PasskeyController) Register(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg, sessions, ok := passkeyRuntime(w)
	if !ok {
		return
	}
	user, ok := sessions.User(r)
	if !ok {
		app.Error(w, app.NewHTTPError(http.StatusUnauthorized, "entre antes de salvar a chave de acesso"))
		return
	}
	challenge, err := consumePasskeyChallenge(w, r, cfg, "register")
	if err != nil {
		app.Error(w, err)
		return
	}
	if challenge.Matricula != "" && challenge.Matricula != user.Matricula {
		app.Error(w, app.NewHTTPError(http.StatusForbidden, "chave de acesso nao pertence a esta sessao"))
		return
	}
	defer r.Body.Close()
	var req attestationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes*4))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		app.Error(w, app.NewHTTPError(http.StatusBadRequest, "json invalido"))
		return
	}
	credentialID, publicKey, signCount, err := verifyAttestationRequest(r, req, challenge.Challenge)
	if err != nil {
		app.Error(w, err)
		return
	}
	now := time.Now().Unix()
	setPasskeyCookie(w, cfg, storedPasskey{
		Version:      1,
		User:         user,
		UserID:       challenge.UserID,
		CredentialID: base64.RawURLEncoding.EncodeToString(credentialID),
		PublicKey:    publicKey,
		SignCount:    signCount,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	app.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (PasskeyController) LoginOptions(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg, _, ok := passkeyRuntime(w)
	if !ok {
		return
	}
	stored, ok := readPasskeyCookie(r, cfg)
	if !ok || stored.CredentialID == "" {
		app.Error(w, app.NewHTTPError(http.StatusNotFound, "nenhuma chave de acesso salva neste navegador"))
		return
	}
	challenge, err := randomBase64URL(32)
	if err != nil {
		app.Error(w, err)
		return
	}
	setPasskeyChallenge(w, cfg, passkeyChallenge{
		Kind:      "login",
		Challenge: challenge,
		Matricula: stored.User.Matricula,
		Expires:   time.Now().Add(passkeyChallengeTTL).Unix(),
	})
	app.JSON(w, http.StatusOK, passkeyOptionsResponse{PublicKey: passkeyRequestOptions{
		Challenge: challenge,
		RPID:      passkeyRPID(r),
		AllowCredentials: []passkeyCredentialDescriptor{
			{Type: "public-key", ID: stored.CredentialID},
		},
		UserVerification: "preferred",
		Timeout:          60_000,
	}})
}

func (PasskeyController) Login(w http.ResponseWriter, r *http.Request) {
	if !app.Method(w, r, http.MethodPost) {
		return
	}
	cfg, sessions, ok := passkeyRuntime(w)
	if !ok {
		return
	}
	stored, ok := readPasskeyCookie(r, cfg)
	if !ok || stored.CredentialID == "" {
		app.Error(w, app.NewHTTPError(http.StatusNotFound, "nenhuma chave de acesso salva neste navegador"))
		return
	}
	challenge, err := consumePasskeyChallenge(w, r, cfg, "login")
	if err != nil {
		app.Error(w, err)
		return
	}
	if challenge.Matricula != "" && challenge.Matricula != stored.User.Matricula {
		app.Error(w, app.NewHTTPError(http.StatusForbidden, "chave de acesso nao pertence a esta sessao"))
		return
	}
	defer r.Body.Close()
	var req assertionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBodyBytes*4))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		app.Error(w, app.NewHTTPError(http.StatusBadRequest, "json invalido"))
		return
	}
	signCount, err := verifyAssertionRequest(r, req, challenge.Challenge, stored)
	if err != nil {
		app.Error(w, err)
		return
	}
	if signCount > stored.SignCount {
		stored.SignCount = signCount
		stored.UpdatedAt = time.Now().Unix()
		setPasskeyCookie(w, cfg, stored)
	}
	sessions.Set(w, stored.User)
	app.JSON(w, http.StatusOK, publicUser(stored.User))
}

func passkeyRuntime(w http.ResponseWriter) (app.Config, app.SessionManager, bool) {
	cfg := app.LoadConfig()
	if strings.TrimSpace(cfg.SessionSecret) == "" {
		app.Error(w, app.NewHTTPError(http.StatusInternalServerError, "SESSION_SECRET nao configurado"))
		return cfg, app.SessionManager{}, false
	}
	return cfg, app.NewSessionManager(cfg), true
}

func verifyAttestationRequest(r *http.Request, req attestationRequest, challenge string) ([]byte, passkeyPublicKey, uint32, error) {
	if req.Type != "public-key" {
		return nil, passkeyPublicKey{}, 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	clientDataJSON, err := decodeBase64URL(req.Response.ClientDataJSON)
	if err != nil {
		return nil, passkeyPublicKey{}, 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	if err := verifyClientData(r, clientDataJSON, webauthnCreateType, challenge); err != nil {
		return nil, passkeyPublicKey{}, 0, err
	}
	rawAttestation, err := decodeBase64URL(req.Response.AttestationObject)
	if err != nil {
		return nil, passkeyPublicKey{}, 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	var attestation attestationObject
	if err := cbor.Unmarshal(rawAttestation, &attestation); err != nil {
		return nil, passkeyPublicKey{}, 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	authData, err := parseRegistrationAuthData(attestation.AuthData)
	if err != nil {
		return nil, passkeyPublicKey{}, 0, err
	}
	if err := verifyRPIDHash(r, authData.RPIDHash); err != nil {
		return nil, passkeyPublicKey{}, 0, err
	}
	if authData.Flags&0x01 == 0 || authData.Flags&0x40 == 0 {
		return nil, passkeyPublicKey{}, 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso nao confirmada")
	}
	return authData.CredentialID, authData.PublicKey, authData.SignCount, nil
}

func verifyAssertionRequest(r *http.Request, req assertionRequest, challenge string, stored storedPasskey) (uint32, error) {
	if req.Type != "public-key" {
		return 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	rawID, err := decodeBase64URL(firstNonEmpty(req.RawID, req.ID))
	if err != nil {
		return 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	expectedID, err := decodeBase64URL(stored.CredentialID)
	if err != nil || !bytes.Equal(rawID, expectedID) {
		return 0, app.NewHTTPError(http.StatusForbidden, "chave de acesso nao reconhecida")
	}
	clientDataJSON, err := decodeBase64URL(req.Response.ClientDataJSON)
	if err != nil {
		return 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	if err := verifyClientData(r, clientDataJSON, webauthnGetType, challenge); err != nil {
		return 0, err
	}
	authenticatorData, err := decodeBase64URL(req.Response.AuthenticatorData)
	if err != nil {
		return 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	authData, err := parseAssertionAuthData(authenticatorData)
	if err != nil {
		return 0, err
	}
	if err := verifyRPIDHash(r, authData.RPIDHash); err != nil {
		return 0, err
	}
	if authData.Flags&0x01 == 0 {
		return 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso nao confirmada")
	}
	signature, err := decodeBase64URL(req.Response.Signature)
	if err != nil {
		return 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	clientHash := sha256.Sum256(clientDataJSON)
	signedData := append(append([]byte{}, authenticatorData...), clientHash[:]...)
	if !verifyES256Signature(stored.PublicKey, signedData, signature) {
		return 0, app.NewHTTPError(http.StatusForbidden, "assinatura da chave de acesso invalida")
	}
	return authData.SignCount, nil
}

func verifyClientData(r *http.Request, raw []byte, expectedType string, expectedChallenge string) error {
	var clientData webauthnClientData
	if err := json.Unmarshal(raw, &clientData); err != nil {
		return app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	if clientData.Type != expectedType || clientData.Challenge != expectedChallenge {
		return app.NewHTTPError(http.StatusForbidden, "desafio de chave de acesso invalido")
	}
	if !samePasskeyOrigin(clientData.Origin, r) {
		return app.NewHTTPError(http.StatusForbidden, "origem da chave de acesso nao permitida")
	}
	return nil
}

func parseRegistrationAuthData(data []byte) (parsedAuthData, error) {
	authData, offset, err := parseAuthDataPrefix(data)
	if err != nil {
		return parsedAuthData{}, err
	}
	if authData.Flags&0x40 == 0 {
		return parsedAuthData{}, app.NewHTTPError(http.StatusBadRequest, "chave de acesso sem credencial")
	}
	if len(data) < offset+18 {
		return parsedAuthData{}, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	offset += 16
	credentialIDLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if credentialIDLength <= 0 || len(data) < offset+credentialIDLength {
		return parsedAuthData{}, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	authData.CredentialID = append([]byte{}, data[offset:offset+credentialIDLength]...)
	offset += credentialIDLength
	publicKey, err := parseCOSEPublicKey(data[offset:])
	if err != nil {
		return parsedAuthData{}, err
	}
	authData.PublicKey = publicKey
	return authData, nil
}

func parseAssertionAuthData(data []byte) (parsedAuthData, error) {
	authData, _, err := parseAuthDataPrefix(data)
	return authData, err
}

func parseAuthDataPrefix(data []byte) (parsedAuthData, int, error) {
	if len(data) < 37 {
		return parsedAuthData{}, 0, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	return parsedAuthData{
		RPIDHash:  append([]byte{}, data[:32]...),
		Flags:     data[32],
		SignCount: binary.BigEndian.Uint32(data[33:37]),
	}, 37, nil
}

func parseCOSEPublicKey(data []byte) (passkeyPublicKey, error) {
	var cose map[int]any
	if err := cbor.Unmarshal(data, &cose); err != nil {
		return passkeyPublicKey{}, app.NewHTTPError(http.StatusBadRequest, "chave de acesso invalida")
	}
	kty, _ := intFromCOSE(cose[1])
	alg, _ := intFromCOSE(cose[3])
	crv, _ := intFromCOSE(cose[-1])
	x, xOK := bytesFromCOSE(cose[-2])
	y, yOK := bytesFromCOSE(cose[-3])
	if kty != 2 || alg != -7 || crv != 1 || !xOK || !yOK {
		return passkeyPublicKey{}, app.NewHTTPError(http.StatusBadRequest, "chave de acesso precisa usar ES256")
	}
	return passkeyPublicKey{
		Alg: alg,
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(x),
		Y:   base64.RawURLEncoding.EncodeToString(y),
	}, nil
}

func verifyES256Signature(publicKey passkeyPublicKey, signedData []byte, signature []byte) bool {
	if publicKey.Alg != -7 || publicKey.Crv != "P-256" {
		return false
	}
	xBytes, err := decodeBase64URL(publicKey.X)
	if err != nil {
		return false
	}
	yBytes, err := decodeBase64URL(publicKey.Y)
	if err != nil {
		return false
	}
	x := new(big.Int).SetBytes(xBytes)
	y := new(big.Int).SetBytes(yBytes)
	if !elliptic.P256().IsOnCurve(x, y) {
		return false
	}
	digest := sha256.Sum256(signedData)
	return ecdsa.VerifyASN1(&ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, digest[:], signature)
}

func verifyRPIDHash(r *http.Request, actual []byte) error {
	expected := sha256.Sum256([]byte(passkeyRPID(r)))
	if !bytes.Equal(actual, expected[:]) {
		return app.NewHTTPError(http.StatusForbidden, "chave de acesso nao pertence a este dominio")
	}
	return nil
}

func consumePasskeyChallenge(w http.ResponseWriter, r *http.Request, cfg app.Config, kind string) (passkeyChallenge, error) {
	defer clearPasskeyChallenge(w, cfg)
	var challenge passkeyChallenge
	if err := readSignedCookie(r, passkeyChallengeCookieName, cfg, &challenge); err != nil {
		return passkeyChallenge{}, app.NewHTTPError(http.StatusBadRequest, "desafio de chave de acesso expirado")
	}
	if challenge.Kind != kind || challenge.Expires < time.Now().Unix() || challenge.Challenge == "" {
		return passkeyChallenge{}, app.NewHTTPError(http.StatusBadRequest, "desafio de chave de acesso expirado")
	}
	return challenge, nil
}

func setPasskeyChallenge(w http.ResponseWriter, cfg app.Config, challenge passkeyChallenge) {
	setSignedCookie(w, passkeyChallengeCookieName, cfg, challenge, passkeyChallengeTTL)
}

func clearPasskeyChallenge(w http.ResponseWriter, cfg app.Config) {
	clearSignedCookie(w, passkeyChallengeCookieName, cfg)
}

func readPasskeyCookie(r *http.Request, cfg app.Config) (storedPasskey, bool) {
	var stored storedPasskey
	if err := readSignedCookie(r, passkeyCookieName, cfg, &stored); err != nil {
		return storedPasskey{}, false
	}
	return stored, stored.Version == 1 && stored.User.Matricula != "" && stored.CredentialID != ""
}

func setPasskeyCookie(w http.ResponseWriter, cfg app.Config, stored storedPasskey) {
	setSignedCookie(w, passkeyCookieName, cfg, stored, passkeyCookieTTL)
}

func setSignedCookie(w http.ResponseWriter, name string, cfg app.Config, value any, ttl time.Duration) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	token := payload + "." + signPasskeyPayload(payload, cfg.SessionSecret)
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func readSignedCookie(r *http.Request, name string, cfg app.Config, target any) error {
	cookie, err := r.Cookie(name)
	if err != nil || cookie.Value == "" {
		return http.ErrNoCookie
	}
	payload, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok || payload == "" || signature == "" {
		return http.ErrNoCookie
	}
	expected := signPasskeyPayload(payload, cfg.SessionSecret)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return http.ErrNoCookie
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func clearSignedCookie(w http.ResponseWriter, name string, cfg app.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func signPasskeyPayload(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomBase64URL(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeBase64URL(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
}

func intFromCOSE(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case uint64:
		return int(typed), true
	default:
		return 0, false
	}
}

func bytesFromCOSE(value any) ([]byte, bool) {
	typed, ok := value.([]byte)
	return typed, ok
}

func passkeyRPID(r *http.Request) string {
	host := requestHost(r)
	host = strings.TrimSpace(strings.Split(host, ",")[0])
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

func requestHost(r *http.Request) string {
	if host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); host != "" {
		return host
	}
	return strings.TrimSpace(r.Host)
}

func samePasskeyOrigin(origin string, r *http.Request) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	expectedScheme := "https"
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		expectedScheme = strings.ToLower(strings.Split(forwardedProto, ",")[0])
	} else if r.TLS == nil && isLocalPasskeyHost(requestHost(r)) {
		expectedScheme = "http"
	}
	return strings.EqualFold(parsed.Scheme, expectedScheme) && strings.EqualFold(parsed.Host, requestHost(r))
}

func isLocalPasskeyHost(host string) bool {
	host = strings.ToLower(host)
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

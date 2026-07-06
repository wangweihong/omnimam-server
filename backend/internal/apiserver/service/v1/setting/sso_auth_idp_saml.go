package setting

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"text/template"
	"time"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/crewjam/saml/xmlenc"
	xrv "github.com/mattermost/xml-roundtrip-validator"
	dsig "github.com/russellhaering/goxmldsig"
	gx509 "github.com/wangweihong/gotoolbox/pkg/certificate/x509"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/randutil"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// 对接收到的sp的SAML请求进行验证回应
func (s *settingService) SAMLProtocolSSOAuth(
	ctx context.Context,
	req *iapiserver.IdpServeSSOAnswerRequest,
) (*bytes.Buffer, error) {
	meta, err := s.store.Settings().GetByName(ctx, iapiserver.SettingKindSSOSamlIdpMetadata)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	covertMeta := iapiserver.IdentityProviderMetadataExtender(*meta)
	rootURL, err := url.Parse(covertMeta.GetEndpoint())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ssoUrl := rootURL.ResolveReference(&url.URL{Path: iapiserver.SsoIdpURLAnswer})
	idp, ed, err := samlEntityDescriptorGenerate(meta)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	r, err := NewIdpAuthnRequest(req, idp)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := r.Validate(ctx, ed, ssoUrl, s.store.ServiceProviders()); err != nil {
		return nil, errors.WithStack(err)
	}

	return r.WriteResponse(ctx)

}

type IdpAuthnRequest struct {
	IDP                     *IdentityProvider
	RemoteAddr              string
	HTTPRequest             *http.Request
	RelayState              string
	RequestBuffer           []byte
	Request                 saml.AuthnRequest
	ServiceProviderMetadata *saml.EntityDescriptor
	SPSSODescriptor         *saml.SPSSODescriptor
	ACSEndpoint             *saml.IndexedEndpoint
	Assertion               *saml.Assertion
	AssertionEl             *etree.Element
	ResponseEl              *etree.Element
	Now                     time.Time
}

func (req IdpAuthnRequest) Validate(
	ctx context.Context,
	ed *saml.EntityDescriptor,
	ssoURL *url.URL,
	spStore store.ServiceProviderStore,
) error {
	if err := xrv.Validate(bytes.NewReader(req.RequestBuffer)); err != nil {
		return errors.Errorf("validate err:%v", err)
	}

	if err := xml.Unmarshal(req.RequestBuffer, &req.Request); err != nil {
		return errors.Errorf("xml unmarshal err:%v", err)
	}

	// We always have exactly one IDP SSO descriptor
	if len(ed.IDPSSODescriptors) != 1 {
		return errors.Errorf("expected exactly one IDP SSO descriptor in IDP metadata")
	}
	idpSsoDescriptor := ed.IDPSSODescriptors[0]
	if idpSsoDescriptor.WantAuthnRequestsSigned != nil && *idpSsoDescriptor.WantAuthnRequestsSigned {
		return errors.Errorf("authn request signature checking is not currently supported")
	}

	mustHaveDestination := idpSsoDescriptor.WantAuthnRequestsSigned != nil && *idpSsoDescriptor.WantAuthnRequestsSigned
	mustHaveDestination = mustHaveDestination || req.Request.Destination != ""
	if mustHaveDestination {
		if req.Request.Destination != ssoURL.String() {
			return errors.Errorf("expected destination to be %q, not %q", ssoURL.String(), req.Request.Destination)
		}
	}

	if req.Request.IssueInstant.Add(saml.MaxIssueDelay).Before(req.Now) {
		return errors.Errorf("request expired at %s",
			req.Request.IssueInstant.Add(saml.MaxIssueDelay))
	}
	if req.Request.Version != "2.0" {
		return errors.Errorf("expected SAML request version 2.0 got %v", req.Request.Version)
	}

	// find the service provider
	serviceProviderID := req.Request.Issuer.Value
	// 从中获取得到对应的sp的配置
	serviceProvider, err := spStore.GetByKey(ctx, iapiserver.SSOProtocolSAML, serviceProviderID)
	if err == os.ErrNotExist {
		return errors.Errorf("cannot handle request from unknown service provider %s", serviceProviderID)
	} else if err != nil {
		return errors.Errorf("cannot find service provider %s: %v", serviceProviderID, err)
	}
	spMetadata, err := samlsp.ParseMetadata([]byte(serviceProvider.SAML.Metadata))
	if err != nil {
		return fmt.Errorf("parse sp metadata error:%v", err)
	}
	req.ServiceProviderMetadata = spMetadata

	// Check that the ACS URL matches an ACS endpoint in the SP metadata.
	if err := req.getACSEndpoint(); err != nil {
		return fmt.Errorf("cannot find assertion consumer service: %v", err)
	}

	return nil
}

func (req IdpAuthnRequest) getACSEndpoint() error {
	if req.Request.AssertionConsumerServiceIndex != "" {
		log.Infof("AssertionConsumerServiceIndex:%v", req.Request.AssertionConsumerServiceIndex)
		for _, spssoDescriptor := range req.ServiceProviderMetadata.SPSSODescriptors {
			for _, spAssertionConsumerService := range spssoDescriptor.AssertionConsumerServices {
				if strconv.Itoa(spAssertionConsumerService.Index) == req.Request.AssertionConsumerServiceIndex {
					spssoDescriptor, spAssertionConsumerService := spssoDescriptor, spAssertionConsumerService

					req.SPSSODescriptor = &spssoDescriptor
					req.ACSEndpoint = &spAssertionConsumerService
					return nil
				}
			}
		}
	}

	if req.Request.AssertionConsumerServiceURL != "" {
		log.Infof("AssertionConsumerServiceURL:%v", req.Request.AssertionConsumerServiceURL)
		for _, spssoDescriptor := range req.ServiceProviderMetadata.SPSSODescriptors {
			for _, spAssertionConsumerService := range spssoDescriptor.AssertionConsumerServices {
				if spAssertionConsumerService.Location == req.Request.AssertionConsumerServiceURL {
					spssoDescriptor, spAssertionConsumerService := spssoDescriptor, spAssertionConsumerService

					req.SPSSODescriptor = &spssoDescriptor
					req.ACSEndpoint = &spAssertionConsumerService
					return nil
				}
			}
		}
	}

	// Some service providers, like the Microsoft Azure AD service provider, issue
	// assertion requests that don't specify an ACS url at all.
	if req.Request.AssertionConsumerServiceURL == "" && req.Request.AssertionConsumerServiceIndex == "" {
		// find a default ACS binding in the metadata that we can use
		for _, spssoDescriptor := range req.ServiceProviderMetadata.SPSSODescriptors {
			for _, spAssertionConsumerService := range spssoDescriptor.AssertionConsumerServices {
				if spAssertionConsumerService.IsDefault != nil && *spAssertionConsumerService.IsDefault {
					switch spAssertionConsumerService.Binding {
					case saml.HTTPPostBinding, saml.HTTPRedirectBinding:
						spssoDescriptor, spAssertionConsumerService := spssoDescriptor, spAssertionConsumerService

						req.SPSSODescriptor = &spssoDescriptor
						req.ACSEndpoint = &spAssertionConsumerService
						return nil
					}
				}
			}
		}

		// if we can't find a default, use *any* ACS binding
		for _, spssoDescriptor := range req.ServiceProviderMetadata.SPSSODescriptors {
			for _, spAssertionConsumerService := range spssoDescriptor.AssertionConsumerServices {
				switch spAssertionConsumerService.Binding {
				case saml.HTTPPostBinding, saml.HTTPRedirectBinding:
					spssoDescriptor, spAssertionConsumerService := spssoDescriptor, spAssertionConsumerService

					req.SPSSODescriptor = &spssoDescriptor
					req.ACSEndpoint = &spAssertionConsumerService
					return nil
				}
			}
		}
	}

	return os.ErrNotExist // no ACS url found or specified
}

func (req IdpAuthnRequest) MakeAssertion(ctx context.Context, ed *saml.EntityDescriptor, session *saml.Session) error {
	attributes := []saml.Attribute{}

	var attributeConsumingService *saml.AttributeConsumingService
	for _, acs := range req.SPSSODescriptor.AttributeConsumingServices {
		if acs.IsDefault != nil && *acs.IsDefault {
			acs := acs

			attributeConsumingService = &acs
			break
		}
	}
	if attributeConsumingService == nil {
		for _, acs := range req.SPSSODescriptor.AttributeConsumingServices {
			acs := acs

			attributeConsumingService = &acs
			break
		}
	}
	if attributeConsumingService == nil {
		attributeConsumingService = &saml.AttributeConsumingService{}
	}

	for _, requestedAttribute := range attributeConsumingService.RequestedAttributes {
		if requestedAttribute.NameFormat == "urn:oasis:names:tc:SAML:2.0:attrname-format:basic" ||
			requestedAttribute.NameFormat == "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified" {
			attrName := requestedAttribute.Name
			attrName = regexp.MustCompile("[^A-Za-z0-9]+").ReplaceAllString(attrName, "")
			switch attrName {
			case "email", "emailaddress":
				attributes = append(attributes, saml.Attribute{
					FriendlyName: requestedAttribute.FriendlyName,
					Name:         requestedAttribute.Name,
					NameFormat:   requestedAttribute.NameFormat,
					Values: []saml.AttributeValue{{
						Type:  "xs:string",
						Value: session.UserEmail,
					}},
				})
			case "name", "fullname", "cn", "commonname":
				attributes = append(attributes, saml.Attribute{
					FriendlyName: requestedAttribute.FriendlyName,
					Name:         requestedAttribute.Name,
					NameFormat:   requestedAttribute.NameFormat,
					Values: []saml.AttributeValue{{
						Type:  "xs:string",
						Value: session.UserCommonName,
					}},
				})
			case "givenname", "firstname":
				attributes = append(attributes, saml.Attribute{
					FriendlyName: requestedAttribute.FriendlyName,
					Name:         requestedAttribute.Name,
					NameFormat:   requestedAttribute.NameFormat,
					Values: []saml.AttributeValue{{
						Type:  "xs:string",
						Value: session.UserGivenName,
					}},
				})
			case "surname", "lastname", "familyname":
				attributes = append(attributes, saml.Attribute{
					FriendlyName: requestedAttribute.FriendlyName,
					Name:         requestedAttribute.Name,
					NameFormat:   requestedAttribute.NameFormat,
					Values: []saml.AttributeValue{{
						Type:  "xs:string",
						Value: session.UserSurname,
					}},
				})
			case "uid", "user", "userid":
				attributes = append(attributes, saml.Attribute{
					FriendlyName: requestedAttribute.FriendlyName,
					Name:         requestedAttribute.Name,
					NameFormat:   requestedAttribute.NameFormat,
					Values: []saml.AttributeValue{{
						Type:  "xs:string",
						Value: session.UserName,
					}},
				})
			}
		}
	}

	if session.UserName != "" {
		attributes = append(attributes, saml.Attribute{
			FriendlyName: "uid",
			Name:         "urn:oid:0.9.2342.19200300.100.1.1",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []saml.AttributeValue{{
				Type:  "xs:string",
				Value: session.UserName,
			}},
		})
	}

	if session.UserEmail != "" {
		attributes = append(attributes, saml.Attribute{
			FriendlyName: "eduPersonPrincipalName",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.6",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []saml.AttributeValue{{
				Type:  "xs:string",
				Value: session.UserEmail,
			}},
		})
	}
	if session.UserSurname != "" {
		attributes = append(attributes, saml.Attribute{
			FriendlyName: "sn",
			Name:         "urn:oid:2.5.4.4",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []saml.AttributeValue{{
				Type:  "xs:string",
				Value: session.UserSurname,
			}},
		})
	}
	if session.UserGivenName != "" {
		attributes = append(attributes, saml.Attribute{
			FriendlyName: "givenName",
			Name:         "urn:oid:2.5.4.42",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []saml.AttributeValue{{
				Type:  "xs:string",
				Value: session.UserGivenName,
			}},
		})
	}

	if session.UserCommonName != "" {
		attributes = append(attributes, saml.Attribute{
			FriendlyName: "cn",
			Name:         "urn:oid:2.5.4.3",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []saml.AttributeValue{{
				Type:  "xs:string",
				Value: session.UserCommonName,
			}},
		})
	}

	if session.UserScopedAffiliation != "" {
		attributes = append(attributes, saml.Attribute{
			FriendlyName: "uid",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.9",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []saml.AttributeValue{{
				Type:  "xs:string",
				Value: session.UserScopedAffiliation,
			}},
		})
	}

	attributes = append(attributes, session.CustomAttributes...)

	if len(session.Groups) != 0 {
		groupMemberAttributeValues := []saml.AttributeValue{}
		for _, group := range session.Groups {
			groupMemberAttributeValues = append(groupMemberAttributeValues, saml.AttributeValue{
				Type:  "xs:string",
				Value: group,
			})
		}
		attributes = append(attributes, saml.Attribute{
			FriendlyName: "eduPersonAffiliation",
			Name:         "urn:oid:1.3.6.1.4.1.5923.1.1.1.1",
			NameFormat:   "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values:       groupMemberAttributeValues,
		})
	}

	if session.SubjectID != "" {
		attributes = append(attributes, saml.Attribute{
			Name:       "urn:oasis:names:tc:SAML:attribute:subject-id",
			NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
			Values: []saml.AttributeValue{
				{
					Type:  "xs:string",
					Value: session.SubjectID,
				},
			},
		})
	}

	// allow for some clock skew in the validity period using the
	// issuer's apparent clock.
	notBefore := req.Now.Add(-1 * saml.MaxClockSkew)
	notOnOrAfterAfter := req.Now.Add(saml.MaxIssueDelay)
	if notBefore.Before(req.Request.IssueInstant) {
		notBefore = req.Request.IssueInstant
		notOnOrAfterAfter = notBefore.Add(saml.MaxIssueDelay)
	}

	nameIDFormat := "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"

	if session.NameIDFormat != "" {
		nameIDFormat = session.NameIDFormat
	}

	req.Assertion = &saml.Assertion{
		ID:           fmt.Sprintf("id-%x", randutil.RandBytes(20)),
		IssueInstant: saml.TimeNow(),
		Version:      "2.0",
		Issuer: saml.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  ed.EntityID,
		},
		Subject: &saml.Subject{
			NameID: &saml.NameID{
				Format:          nameIDFormat,
				NameQualifier:   ed.EntityID,
				SPNameQualifier: req.ServiceProviderMetadata.EntityID,
				Value:           session.NameID,
			},
			SubjectConfirmations: []saml.SubjectConfirmation{
				{
					Method: "urn:oasis:names:tc:SAML:2.0:cm:bearer",
					SubjectConfirmationData: &saml.SubjectConfirmationData{
						Address:      req.HTTPRequest.RemoteAddr,
						InResponseTo: req.Request.ID,
						NotOnOrAfter: req.Now.Add(saml.MaxIssueDelay),
						Recipient:    req.ACSEndpoint.Location,
					},
				},
			},
		},
		Conditions: &saml.Conditions{
			NotBefore:    notBefore,
			NotOnOrAfter: notOnOrAfterAfter,
			AudienceRestrictions: []saml.AudienceRestriction{
				{
					Audience: saml.Audience{Value: req.ServiceProviderMetadata.EntityID},
				},
			},
		},
		AuthnStatements: []saml.AuthnStatement{
			{
				AuthnInstant: session.CreateTime,
				SessionIndex: session.Index,
				SubjectLocality: &saml.SubjectLocality{
					Address: req.HTTPRequest.RemoteAddr,
				},
				AuthnContext: saml.AuthnContext{
					AuthnContextClassRef: &saml.AuthnContextClassRef{
						Value: "urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport",
					},
				},
			},
		},
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: attributes,
			},
		},
	}

	return nil
}

// MakeResponse creates and assigns a new SAML response in ResponseEl. `Assertion` must
// be non-nil. If MakeAssertionEl() has not been called, this function calls it for
// you.
func (req *IdpAuthnRequest) MakeResponse() error {
	if req.AssertionEl == nil {
		if err := req.MakeAssertionEl(); err != nil {
			return err
		}
	}

	response := &saml.Response{
		Destination:  req.ACSEndpoint.Location,
		ID:           fmt.Sprintf("id-%x", randutil.RandBytes(20)),
		InResponseTo: req.Request.ID,
		IssueInstant: req.Now,
		Version:      "2.0",
		Issuer: &saml.Issuer{
			Format: "urn:oasis:names:tc:SAML:2.0:nameid-format:entity",
			Value:  req.IDP.RootURL.String(),
			//	Value:  req.IDP.MetadataURL.String(),
		},
		Status: saml.Status{
			StatusCode: saml.StatusCode{
				Value: saml.StatusSuccess,
			},
		},
	}

	responseEl := response.Element()
	responseEl.AddChild(req.AssertionEl) // AssertionEl either an EncryptedAssertion or Assertion element

	// Sign the response element (we've already signed the Assertion element)
	{
		signingContext, err := req.signingContext()
		if err != nil {
			return errors.WithStack(err)
		}

		signedResponseEl, err := signingContext.SignEnveloped(responseEl)
		if err != nil {
			return errors.WithStack(err)
		}

		sigEl := signedResponseEl.ChildElements()[len(signedResponseEl.ChildElements())-1]
		response.Signature = sigEl
		responseEl = response.Element()
		responseEl.AddChild(req.AssertionEl)
	}

	req.ResponseEl = responseEl
	return nil
}
func (req *IdpAuthnRequest) getSPEncryptionCert() (*x509.Certificate, error) {
	certStr := ""
	for _, keyDescriptor := range req.SPSSODescriptor.KeyDescriptors {
		if keyDescriptor.Use == "encryption" {
			certStr = keyDescriptor.KeyInfo.X509Data.X509Certificates[0].Data
			break
		}
	}

	// If there are no certs explicitly labeled for encryption, return the first
	// non-empty cert we find.
	if certStr == "" {
		for _, keyDescriptor := range req.SPSSODescriptor.KeyDescriptors {
			if keyDescriptor.Use == "" && len(keyDescriptor.KeyInfo.X509Data.X509Certificates) != 0 &&
				keyDescriptor.KeyInfo.X509Data.X509Certificates[0].Data != "" {
				certStr = keyDescriptor.KeyInfo.X509Data.X509Certificates[0].Data
				break
			}
		}
	}

	if certStr == "" {
		return nil, os.ErrNotExist
	}

	// cleanup whitespace and re-encode a PEM
	certStr = regexp.MustCompile(`\s+`).ReplaceAllString(certStr, "")
	certBytes, err := base64.StdEncoding.DecodeString(certStr)
	if err != nil {
		return nil, errors.Errorf("cannot decode certificate base64: %v", err)
	}
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, errors.Errorf("cannot parse certificate: %v", err)
	}
	return cert, nil
}

const canonicalizerPrefixList = ""

// MakeAssertionEl sets `AssertionEl` to a signed, possibly encrypted, version of `Assertion`.
func (req *IdpAuthnRequest) MakeAssertionEl() error {
	signingContext, err := req.signingContext()
	if err != nil {
		return err
	}

	assertionEl := req.Assertion.Element()

	signedAssertionEl, err := signingContext.SignEnveloped(assertionEl)
	if err != nil {
		return err
	}

	sigEl := signedAssertionEl.Child[len(signedAssertionEl.Child)-1]
	req.Assertion.Signature = sigEl.(*etree.Element)
	signedAssertionEl = req.Assertion.Element()

	certBuf, err := req.getSPEncryptionCert()
	if err == os.ErrNotExist {
		req.AssertionEl = signedAssertionEl
		return nil
	} else if err != nil {
		return err
	}

	var signedAssertionBuf []byte
	{
		doc := etree.NewDocument()
		doc.SetRoot(signedAssertionEl)
		signedAssertionBuf, err = doc.WriteToBytes()
		if err != nil {
			return err
		}
	}

	encryptor := xmlenc.OAEP()
	encryptor.BlockCipher = xmlenc.AES128CBC
	encryptor.DigestMethod = &xmlenc.SHA1
	encryptedDataEl, err := encryptor.Encrypt(certBuf, signedAssertionBuf, nil)
	if err != nil {
		return err
	}
	encryptedDataEl.CreateAttr("Type", "http://www.w3.org/2001/04/xmlenc#Element")

	encryptedAssertionEl := etree.NewElement("saml:EncryptedAssertion")
	encryptedAssertionEl.AddChild(encryptedDataEl)
	req.AssertionEl = encryptedAssertionEl

	return nil
}

// signingContext will create a signing context for the request.
func (req *IdpAuthnRequest) signingContext() (*dsig.SigningContext, error) {
	// Create a cert chain based off of the IDP cert and its intermediates.
	certificates := [][]byte{req.IDP.Certificate.Raw}
	for _, cert := range req.IDP.Intermediates {
		certificates = append(certificates, cert.Raw)
	}

	var signingContext *dsig.SigningContext
	var err error
	// If signer is set, use it instead of the private key.
	if req.IDP.Signer != nil {
		signingContext, err = dsig.NewSigningContext(req.IDP.Signer, certificates)
		if err != nil {
			return nil, err
		}
	} else {
		keyPair := tls.Certificate{
			Certificate: certificates,
			PrivateKey:  req.IDP.Key,
			Leaf:        req.IDP.Certificate,
		}
		keyStore := dsig.TLSCertKeyStore(keyPair)

		signingContext = dsig.NewDefaultSigningContext(keyStore)
	}

	// Default to using SHA1 if the signature method isn't set.
	signatureMethod := req.IDP.SignatureMethod
	if signatureMethod == "" {
		signatureMethod = dsig.RSASHA1SignatureMethod
	}

	signingContext.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList(canonicalizerPrefixList)
	if err := signingContext.SetSignatureMethod(signatureMethod); err != nil {
		return nil, err
	}

	return signingContext, nil
}

func (req IdpAuthnRequest) WriteResponse(ctx context.Context) (*bytes.Buffer, error) {
	form, err := req.PostBinding()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tmpl := template.Must(template.New("saml-post-form").Parse(`<html>` +
		`<form method="post" action="{{.URL}}" id="SAMLResponseForm">` +
		`<input type="hidden" name="SAMLResponse" value="{{.SAMLResponse}}" />` +
		`<input type="hidden" name="RelayState" value="{{.RelayState}}" />` +
		`<input id="SAMLSubmitButton" type="submit" value="Continue" />` +
		`</form>` +
		`<script>document.getElementById('SAMLSubmitButton').style.visibility='hidden';</script>` +
		`<script>document.getElementById('SAMLResponseForm').submit();</script>` +
		`</html>`))

	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, form); err != nil {
		return nil, errors.WithStack(err)
	}
	return buf, nil
	// if _, err := io.Copy(w, buf); err != nil {
	// 	return err
	// }
	// w.Header().Set("Content-Type", "text/html")
	// return nil
}

func (req *IdpAuthnRequest) PostBinding() (saml.IdpAuthnRequestForm, error) {
	var form saml.IdpAuthnRequestForm

	if req.ResponseEl == nil {
		if err := req.MakeResponse(); err != nil {
			return form, err
		}
	}

	doc := etree.NewDocument()
	doc.SetRoot(req.ResponseEl)
	responseBuf, err := doc.WriteToBytes()
	if err != nil {
		return form, err
	}

	if req.ACSEndpoint.Binding != saml.HTTPPostBinding {
		return form, fmt.Errorf("%s: unsupported binding %s",
			req.ServiceProviderMetadata.EntityID,
			req.ACSEndpoint.Binding)
	}

	form.URL = req.ACSEndpoint.Location
	form.SAMLResponse = base64.StdEncoding.EncodeToString(responseBuf)
	form.RelayState = req.RelayState

	return form, nil
}

func NewIdpAuthnRequest(r *iapiserver.IdpServeSSOAnswerRequest, idp *IdentityProvider) (*IdpAuthnRequest, error) {
	req := &IdpAuthnRequest{
		IDP:           idp,
		RemoteAddr:    r.RemoteAddr,
		Now:           time.Now(),
		RequestBuffer: r.DecodedSAMLRequest,
		RelayState:    r.RelayState,
		HTTPRequest:   r.Req,
	}

	return req, nil
}

type IdentityProvider struct {
	Key             crypto.PrivateKey
	Signer          crypto.Signer
	Certificate     *x509.Certificate
	Intermediates   []*x509.Certificate
	RootURL         url.URL
	MetadataURL     url.URL
	SSOURL          url.URL
	LogoutURL       url.URL
	SignatureMethod string
	ValidDuration   *time.Duration
}

func xmlIdpSamlGenerate(meta *iapiserver.Setting) (string, error) {
	_, ed, err := samlEntityDescriptorGenerate(meta)
	if err != nil {
		return "", err
	}

	buf, err := xml.MarshalIndent(ed, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func samlEntityDescriptorGenerate(meta *iapiserver.Setting) (*IdentityProvider, *saml.EntityDescriptor, error) {
	idpMeta := iapiserver.IdentityProviderMetadataExtender(*meta)

	keyPair, Leaf, err := gx509.ParseCert(idpMeta.GetCert(), idpMeta.GetKey())
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	rootURL, err := url.Parse(idpMeta.GetEndpoint())
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	ssoURL := rootURL.ResolveReference(&url.URL{Path: iapiserver.SsoIdpURLAnswer})
	//	sloURL := rootURL.ResolveReference(&url.URL{Path: slo_uri})
	validDuration := iapiserver.DefaultCertificateExpireDuration
	certStr := base64.StdEncoding.EncodeToString(Leaf.Raw)

	ed := &saml.EntityDescriptor{
		EntityID:      rootURL.String(),
		ValidUntil:    time.Now().Add(validDuration),
		CacheDuration: validDuration,
		IDPSSODescriptors: []saml.IDPSSODescriptor{
			{
				SSODescriptor: saml.SSODescriptor{
					RoleDescriptor: saml.RoleDescriptor{
						ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
						KeyDescriptors: []saml.KeyDescriptor{
							{
								Use: "signing",
								KeyInfo: saml.KeyInfo{
									X509Data: saml.X509Data{
										X509Certificates: []saml.X509Certificate{
											{Data: certStr},
										},
									},
								},
							},
							{
								Use: "encryption",
								KeyInfo: saml.KeyInfo{
									X509Data: saml.X509Data{
										X509Certificates: []saml.X509Certificate{
											{Data: certStr},
										},
									},
								},
								EncryptionMethods: []saml.EncryptionMethod{
									{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes128-cbc"},
									{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes192-cbc"},
									{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes256-cbc"},
									{Algorithm: "http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"},
								},
							},
						},
					},
					NameIDFormats: []saml.NameIDFormat{
						saml.NameIDFormat("urn:oasis:names:tc:SAML:2.0:nameid-format:transient"),
					},
				},
				SingleSignOnServices: []saml.Endpoint{
					{
						Binding:  saml.HTTPRedirectBinding,
						Location: ssoURL.String(),
					},
					{
						Binding:  saml.HTTPPostBinding,
						Location: ssoURL.String(),
					},
				},
			},
		},
	}

	//ed.IDPSSODescriptors[0].SSODescriptor.SingleLogoutServices = []saml.Endpoint{
	//	{
	//		Binding:  saml.HTTPRedirectBinding,
	//		Location: sloURL.String(),
	//	},
	//}

	idp := &IdentityProvider{
		Key:           keyPair.PrivateKey,
		Signer:        nil,
		Certificate:   keyPair.Leaf,
		Intermediates: nil,
		RootURL:       *rootURL,
		MetadataURL:   *rootURL.ResolveReference(&url.URL{Path: iapiserver.SsoIdpURLMetadata}),
		SSOURL:        *rootURL.ResolveReference(&url.URL{Path: iapiserver.SsoIdpURLAnswer}),
		//	LogoutURL:       *rootURL.ResolveReference(&url.URL{Path: iapiserver.UriSLO}),
		SignatureMethod: "",
	}

	return idp, ed, nil
}

func xmlSpSamlGenerate(setting *iapiserver.Setting) (string, error) {
	meta := iapiserver.ServiceProviderMetadataExtender(*setting)

	cert, err := base64.StdEncoding.DecodeString(meta.GetCert())
	if err != nil {
		return "", err
	}

	key, err := base64.StdEncoding.DecodeString(meta.GetKey())
	if err != nil {
		return "", err
	}

	keyPair, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		return "", errors.WithStack(err)
	}

	keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return "", errors.WithStack(err)
	}

	rootURL, err := url.Parse(meta.GetEndpoint())
	if err != nil {
		return "", errors.WithStack(err)
	}
	acsURL := rootURL.ResolveReference(&url.URL{Path: iapiserver.SsoSpURLSamlACS})
	//sloURL := rootURL.ResolveReference(&url.URL{Path: "/v1/authentication/saml/slo"})
	//signatureMethod := RSASHA1SignatureMethod
	validDuration := iapiserver.DefaultCertificateExpireDuration
	wantAssertionsSigned := true
	authnRequestsSigned := true
	validUntil := time.Now().Add(validDuration)
	certBytes := keyPair.Leaf.Raw
	var keyDescriptors []saml.KeyDescriptor
	keyDescriptors = []saml.KeyDescriptor{
		{
			Use: "encryption",
			KeyInfo: saml.KeyInfo{
				X509Data: saml.X509Data{
					X509Certificates: []saml.X509Certificate{
						{Data: base64.StdEncoding.EncodeToString(certBytes)},
					},
				},
			},
			EncryptionMethods: []saml.EncryptionMethod{
				{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes128-cbc"},
				{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes192-cbc"},
				{Algorithm: "http://www.w3.org/2001/04/xmlenc#aes256-cbc"},
				{Algorithm: "http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"},
			},
		},
	}
	keyDescriptors = append(keyDescriptors, saml.KeyDescriptor{
		Use: "signing",
		KeyInfo: saml.KeyInfo{
			X509Data: saml.X509Data{
				X509Certificates: []saml.X509Certificate{
					{Data: base64.StdEncoding.EncodeToString(certBytes)},
				},
			},
		},
	})
	//sloEndpoints := make([]saml.Endpoint, len(sp.LogoutBindings))
	//for i, binding := range LogoutBindings {
	//	sloEndpoints[i] = saml.Endpoint{
	//		Binding:          binding,
	//		Location:         sp.SloURL.String(),
	//		ResponseLocation: sp.SloURL.String(),
	//	}
	//}
	EntityID := ""

	AuthnNameIDFormat := saml.NameIDFormat(meta.GetAuthNameIdFormat())
	ed := &saml.EntityDescriptor{
		EntityID:   stringutil.FirstSet(EntityID, rootURL.String()),
		ValidUntil: validUntil,

		SPSSODescriptors: []saml.SPSSODescriptor{
			{
				SSODescriptor: saml.SSODescriptor{
					RoleDescriptor: saml.RoleDescriptor{
						ProtocolSupportEnumeration: "urn:oasis:names:tc:SAML:2.0:protocol",
						KeyDescriptors:             keyDescriptors,
						ValidUntil:                 &validUntil,
					},
					//	SingleLogoutServices: sloEndpoints,
					NameIDFormats: []saml.NameIDFormat{AuthnNameIDFormat},
				},
				AuthnRequestsSigned:  &authnRequestsSigned,
				WantAssertionsSigned: &wantAssertionsSigned,

				AssertionConsumerServices: []saml.IndexedEndpoint{
					{
						Binding:  saml.HTTPPostBinding,
						Location: acsURL.String(),
						Index:    1,
					},
				},
			},
		},
	}
	buf, err := xml.MarshalIndent(ed, "", "  ")
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

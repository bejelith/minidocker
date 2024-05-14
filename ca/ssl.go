package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

var baseName = pkix.Name{
	Organization:  []string{"Company, INC."},
	Country:       []string{"US"},
	Province:      []string{"CA"},
	Locality:      []string{"SF"},
	StreetAddress: []string{""},
	PostalCode:    []string{""},
}

func main() {
	cax509, _, caPK, err := getCA()
	if err != nil {
		log.Printf("can not create CA: %v\n", err)
		os.Exit(1)
	}

	if _, _, err := serverCert(caPK, cax509); err != nil {
		log.Printf("error creating server certificate: %v\n", err)
		os.Exit(1)
	}

	for _, user := range []string{"user1", "user2", "user3", "anonymous"} {
		if _, _, err := clientCert(user, caPK, cax509); err != nil {
			log.Printf("error creating certificate for user %s: %v\n", user, err)
			os.Exit(1)
		}
	}
}

func getRSA() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 4096)
}

func getCA() (ca *x509.Certificate, caBytes []byte, privateKey *rsa.PrivateKey, err error) {

	privateKey, err = getRSA()
	if err != nil {
		return nil, nil, nil, err
	}

	ca = &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization:  []string{"ORGANIZATION_NAME"},
			Country:       []string{"COUNTRY_CODE"},
			Province:      []string{"PROVINCE"},
			Locality:      []string{"CITY"},
			StreetAddress: []string{"ADDRESS"},
			PostalCode:    []string{"POSTAL_CODE"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	pub := &privateKey.PublicKey
	caBytes, err = x509.CreateCertificate(rand.Reader, ca, ca, pub, privateKey)
	if err != nil {
		return
	}

	// Public key
	certOut, err := os.Create("ca.crt")
	if err != nil {
		return
	}
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	if err != nil {
		return
	}
	certOut.Close()
	log.Print("written cert.pem\n")

	// Private key
	keyOut, err := os.OpenFile("ca.key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err != nil {
		return
	}
	keyOut.Close()
	log.Print("written key.pem\n")

	return
}

func serverCert(caPrivKey *rsa.PrivateKey, ca *x509.Certificate) (pk *rsa.PrivateKey, cert_b []byte, err error) {

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject:      baseName,
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"localhost", "example.com", "127.0.0.1"},
	}

	pk, err = getRSA()
	if err != nil {
		return
	}
	cert_b, err = x509.CreateCertificate(rand.Reader, cert, ca, &pk.PublicKey, caPrivKey)
	if err != nil {
		return
	}

	// Public key
	certOut, err := os.Create("server.crt")
	if err != nil {
		return
	}
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert_b})
	if err != nil {
		return
	}
	certOut.Close()
	log.Print("written server.crt\n")

	// Private key
	keyOut, err := os.OpenFile("server.key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
	if err != nil {
		return
	}
	keyOut.Close()
	log.Print("written server.key\n")
	return pk, cert_b, err
}

func clientCert(name string, caPrivKey *rsa.PrivateKey, ca *x509.Certificate) (pk *rsa.PrivateKey, cert_b []byte, err error) {
	subject := baseName
	subject.CommonName = name
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject:      subject,
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	pk, err = getRSA()
	if err != nil {
		return
	}
	cert_b, err = x509.CreateCertificate(rand.Reader, cert, ca, &pk.PublicKey, caPrivKey)
	if err != nil {
		return
	}

	// Public key
	certFileName := fmt.Sprintf("client_%s.crt", name)
	certOut, err := os.Create(certFileName)
	if err != nil {
		return
	}
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert_b})
	if err != nil {
		return
	}
	certOut.Close()
	log.Printf("written %s\n", certFileName)

	// Private key
	keyFileName := fmt.Sprintf("client_%s.key", name)
	keyOut, err := os.OpenFile(keyFileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	err = pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
	if err != nil {
		return
	}
	keyOut.Close()
	log.Printf("written %s\n", keyFileName)
	return pk, cert_b, err
}

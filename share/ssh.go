package chshare

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/armon/go-socks5"
	"github.com/jpillora/sizestr"
	"golang.org/x/crypto/ssh"
)

func GenerateKey(seed, keyType, keySize string) ([]byte, error) {
	var r io.Reader
	if seed == "" {
		r = rand.Reader
	} else {
		r = NewDetermRand([]byte(seed))
	}
	if keyType == "" || keyType == "ECDSA" {
		priv, err := ecdsa.GenerateKey(elliptic.P256(), r)
		if err != nil {
			return nil, err
		}
		b, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			return nil, fmt.Errorf("Unable to marshal ECDSA private key: %v", err)
		}

		return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}), nil

	} else if keyType == "rsa" {
		var size int
		if keySize != "" {
			size, _ = strconv.Atoi(keySize)
			if size < 2048 {
				return nil, fmt.Errorf("RSA keysize too small")
			}
		} else {
			size = 4096
		}
		priv, err := rsa.GenerateKey(r, size)
		if err != nil {
			return nil, err
		}
		b := x509.MarshalPKCS1PrivateKey(priv)
		return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: b}), nil
	}

	return nil, nil
}

func FingerprintKey(k ssh.PublicKey) string {
	bytes := md5.Sum(k.Marshal())
	strbytes := make([]string, len(bytes))
	for i, b := range bytes {
		strbytes[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(strbytes, ":")
}

func HandleTCPStream(l *Logger, connStats *ConnStats, src io.ReadWriteCloser, remote string, dial FnDial) {
	if dial == nil {
		dial = net.Dial
	}
	dst, err := dial("tcp", remote)
	if err != nil {
		l.Debugf("Remote failed (%s)", err)
		src.Close()
		return
	}
	connStats.Open()
	l.Debugf("%s: Open", connStats)
	s, r := Pipe(src, dst)
	connStats.Close()
	l.Debugf("%s: Close (sent %s received %s)", connStats, sizestr.ToString(s), sizestr.ToString(r))
}

func HandleSocksStream(l *Logger, server *socks5.Server, connStats *ConnStats, src io.ReadWriteCloser) {
	conn := NewRWCConn(src)
	connStats.Open()
	l.Debugf("%s Opening", connStats)
	err := server.ServeConn(conn)
	connStats.Close()

	if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
		l.Debugf("%s: Closed (error: %s)", connStats, err)
	} else {
		l.Debugf("%s: Closed", connStats)
	}
}

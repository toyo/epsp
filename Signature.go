package epsp

import (
	"crypto"
	"crypto/md5" // #nosec G501
	"crypto/rsa"
	"crypto/sha1" // #nosec G505
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/pkg/errors"
)

// DecryptKey は、鍵を復号します
func DecryptKey(key []byte) (*rsa.PublicKey, error) {
	keyblock, _ := pem.Decode(key)
	if keyblock == nil {
		return nil, errors.New("鍵書式異常")
	}
	if keyblock.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("鍵種類異常 : %s", keyblock.Type)
	}
	keyInterface, err := x509.ParsePKIXPublicKey(keyblock.Bytes)
	if err != nil {
		return nil, err
	}
	rsakey, ok := keyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("サーバ公開鍵がRSAではない")
	}
	return rsakey, nil
}

// KeySignatureCheck は、鍵署名を照合します
func KeySignatureCheck(key *rsa.PublicKey, pubKey, keySig, keyExpDate string) (*rsa.PublicKey, error) {

	keySignature, err := base64.StdEncoding.DecodeString(keySig)
	if err != nil {
		return nil, errors.Wrap(err, "鍵署名BASE64デコード不可")
	}
	b, err := base64.StdEncoding.DecodeString(pubKey)
	if err != nil {
		return nil, errors.Wrap(err, `鍵BASE64デコード不可`)
	}
	keytokenHasher := sha1.New() // #nosec G401

	if _, err = keytokenHasher.Write(b); err != nil {
		return nil, errors.Wrap(err, `鍵BASE64ハッシュ不可`)
	}
	if _, err = keytokenHasher.Write([]byte(keyExpDate)); err != nil {
		return nil, errors.Wrap(err, `鍵日付ハッシュ不可`)
	}

	if err = rsa.VerifyPKCS1v15(key, crypto.SHA1, keytokenHasher.Sum(nil), keySignature); err != nil {
		return nil, errors.Wrap(err, `鍵署名不一致`)
	}
	keyInterface, err := x509.ParsePKIXPublicKey(b)
	if err != nil {
		return nil, errors.Wrap(err, `公開鍵を解析できません`)
	}
	peerPubKey, ok := keyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.Wrap(err, `公開鍵がRSA公開鍵ではありません`)
	}
	return peerPubKey, nil
}

// DataSignatureCheck は、データ署名を照合します
func DataSignatureCheck(key *rsa.PublicKey, dataSig, expDate, dataBody string) error {
	dataSignature, err := base64.StdEncoding.DecodeString(dataSig)
	if err != nil {
		return errors.Wrap(err, "データ署名BASE64デコード不可, "+dataSig)
	}
	dataBodyhasher := md5.New() // #nosec G401
	if _, err = dataBodyhasher.Write([]byte(dataBody)); err != nil {
		return errors.Wrap(err, `データハッシュ不可`)
	}

	tokenHasher := sha1.New() // #nosec G401
	if _, err = tokenHasher.Write([]byte(expDate)); err != nil {
		return errors.Wrap(err, `鍵日付ハッシュ不可`)
	}
	if _, err = tokenHasher.Write([]byte(dataBodyhasher.Sum(nil))); err != nil {
		return errors.Wrap(err, `データハッシュの再ハッシュ不可`)
	}

	if err = rsa.VerifyPKCS1v15(key, crypto.SHA1, tokenHasher.Sum(nil), dataSignature); err != nil {
		return errors.Wrap(err, "データ署名不一致")
	}
	return nil // データ署名確認
}

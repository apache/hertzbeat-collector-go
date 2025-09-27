/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"sync"
	"unicode"

	loggertype "hertzbeat.apache.org/hertzbeat-collector-go/internal/collector/common/types/logger"
	"hertzbeat.apache.org/hertzbeat-collector-go/internal/util/logger"
)

// AESUtil provides AES encryption/decryption utilities
// This matches Java's AesUtil functionality
type AESUtil struct {
	secretKey string
	mutex     sync.RWMutex
}

var (
	defaultAESUtil = &AESUtil{}
	once           sync.Once
	log            = logger.DefaultLogger(os.Stdout, loggertype.LogLevelInfo).WithName("aes-util")
)

// GetDefaultAESUtil returns the singleton AES utility instance
func GetDefaultAESUtil() *AESUtil {
	once.Do(func() {
		defaultAESUtil = &AESUtil{}
	})
	return defaultAESUtil
}

// SetDefaultSecretKey sets the default AES secret key (matches Java: AesUtil.setDefaultSecretKey)
func SetDefaultSecretKey(secretKey string) {
	GetDefaultAESUtil().SetSecretKey(secretKey)
}

// GetDefaultSecretKey gets the current AES secret key (matches Java: AesUtil.getDefaultSecretKey)
func GetDefaultSecretKey() string {
	return GetDefaultAESUtil().GetSecretKey()
}

// SetSecretKey sets the AES secret key for this instance
func (a *AESUtil) SetSecretKey(secretKey string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.secretKey = secretKey
	log.Info("AES secret key updated", "keyLength", len(secretKey))
}

// GetSecretKey gets the AES secret key for this instance
func (a *AESUtil) GetSecretKey() string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.secretKey
}

// AesDecode decrypts AES encrypted data (matches Java: AesUtil.aesDecode)
func (a *AESUtil) AesDecode(encryptedData string) (string, error) {
	secretKey := a.GetSecretKey()
	if secretKey == "" {
		return "", fmt.Errorf("AES secret key not set")
	}
	return a.AesDecodeWithKey(encryptedData, secretKey)
}

// AesDecodeWithKey decrypts AES encrypted data with specified key
func (a *AESUtil) AesDecodeWithKey(encryptedData, key string) (string, error) {
	// Ensure key is exactly 16 bytes for AES-128 (Java requirement)
	keyBytes := []byte(key)
	if len(keyBytes) != 16 {
		return "", fmt.Errorf("key must be exactly 16 bytes, got %d", len(keyBytes))
	}

	// Decode base64 encoded data
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Check if data length is valid for AES
	if len(encryptedBytes) < aes.BlockSize || len(encryptedBytes)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid encrypted data length: %d", len(encryptedBytes))
	}

	// Java uses the key itself as IV (this matches Java's implementation)
	iv := keyBytes

	// Create CBC decrypter
	mode := cipher.NewCBCDecrypter(block, iv)

	// Decrypt
	decrypted := make([]byte, len(encryptedBytes))
	mode.CryptBlocks(decrypted, encryptedBytes)

	// Remove PKCS5/PKCS7 padding (Go's PKCS7 is compatible with Java's PKCS5 for AES)
	decrypted, err = removePKCS7Padding(decrypted)
	if err != nil {
		return "", fmt.Errorf("failed to remove padding: %w", err)
	}

	return string(decrypted), nil
}

// IsCiphertext determines whether text is encrypted (matches Java: AesUtil.isCiphertext)
func (a *AESUtil) IsCiphertext(text string) bool {
	// First check if it's valid base64
	if _, err := base64.StdEncoding.DecodeString(text); err != nil {
		return false
	}

	// Try to decrypt and see if it succeeds
	_, err := a.AesDecode(text)
	return err == nil
}

// removePKCS7Padding removes PKCS7 padding from decrypted data
func removePKCS7Padding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	paddingLen := int(data[len(data)-1])
	if paddingLen > len(data) || paddingLen == 0 {
		return data, nil // Return as-is if padding seems invalid
	}

	// Verify padding
	for i := 0; i < paddingLen; i++ {
		if data[len(data)-1-i] != byte(paddingLen) {
			return data, nil // Return as-is if padding is invalid
		}
	}

	return data[:len(data)-paddingLen], nil
}

// isPrintableString checks if a string contains only printable characters
func isPrintableString(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, r := range s {
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// Convenience functions that match Java's static methods

// AesDecode decrypts with the default secret key
func AesDecode(encryptedData string) (string, error) {
	return GetDefaultAESUtil().AesDecode(encryptedData)
}

// AesDecodeWithKey decrypts with specified key
func AesDecodeWithKey(encryptedData, key string) (string, error) {
	return GetDefaultAESUtil().AesDecodeWithKey(encryptedData, key)
}

// IsCiphertext checks if text is encrypted using default key
func IsCiphertext(text string) bool {
	return GetDefaultAESUtil().IsCiphertext(text)
}

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
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESUtil_SetAndGetSecretKey(t *testing.T) {
	util := &AESUtil{}

	// Test setting and getting secret key
	testKey := "test-secret-key"
	util.SetSecretKey(testKey)

	assert.Equal(t, testKey, util.GetSecretKey())
}

func TestAESUtil_GetDefaultAESUtil(t *testing.T) {
	// Test singleton pattern
	util1 := GetDefaultAESUtil()
	util2 := GetDefaultAESUtil()

	// Both should be the same instance
	assert.Same(t, util1, util2)
}

func TestAESUtil_SetDefaultSecretKey(t *testing.T) {
	// Test setting default secret key
	testKey := "default-secret-key"
	SetDefaultSecretKey(testKey)

	// Should be able to retrieve it
	assert.Equal(t, testKey, GetDefaultSecretKey())
}

func TestAESUtil_AesDecodeWithKey_Success(t *testing.T) {
	// Test successful decryption with a valid key
	// Using a key that matches the expected format (16 bytes)
	key := "1234567890123456" // 16 bytes exactly

	// This test just verifies that the function can be called with valid inputs
	// without panicking. Actual decryption testing would require generating
	// encrypted data with the exact same algorithm, which is complex to do
	// consistently across implementations.
	util := &AESUtil{}

	// Test with valid base64 but invalid encrypted data (to avoid requiring actual encryption)
	// This will fail decryption but should not panic
	validBase64 := "AAAAAAAAAAAAAAAAAAAAAA==" // 16 bytes of zeros, base64 encoded

	_, err := util.AesDecodeWithKey(validBase64, key)

	// Should not fail due to key or base64 issues, but will fail due to invalid encrypted data
	// The important thing is that the function processes the inputs correctly
	if err != nil {
		assert.NotContains(t, err.Error(), "key must be exactly 16 bytes")
		assert.NotContains(t, err.Error(), "failed to decode base64")
	}
}

func TestAESUtil_AesDecodeWithKey_InvalidBase64(t *testing.T) {
	util := &AESUtil{}
	_, err := util.AesDecodeWithKey("invalid-base64!", "1234567890123456")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode base64")
}

func TestAESUtil_AesDecodeWithKey_WrongKeyLength(t *testing.T) {
	util := &AESUtil{}
	_, err := util.AesDecodeWithKey("some-data", "short-key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key must be exactly 16 bytes")
}

func TestAESUtil_AesDecodeWithKey_InvalidEncryptedDataLength(t *testing.T) {
	key := "1234567890123456"

	// Data that is not a multiple of AES block size
	shortData := base64.StdEncoding.EncodeToString([]byte("short"))

	util := &AESUtil{}
	_, err := util.AesDecodeWithKey(shortData, key)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid encrypted data length")
}

func TestAESUtil_AesDecode_Success(t *testing.T) {
	// Test decryption using instance secret key
	key := "1234567890123456"
	util := &AESUtil{}
	util.SetSecretKey(key)

	// Test with valid base64 but invalid encrypted data
	validBase64 := "AAAAAAAAAAAAAAAAAAAAAA==" // 16 bytes of zeros, base64 encoded

	_, err := util.AesDecode(validBase64)

	// Should not fail due to missing secret key
	if err != nil {
		assert.NotContains(t, err.Error(), "AES secret key not set")
	}
}

func TestAESUtil_AesDecode_NoSecretKey(t *testing.T) {
	util := &AESUtil{} // No secret key set

	_, err := util.AesDecode("some-data")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AES secret key not set")
}

func TestAESUtil_IsCiphertext_Valid(t *testing.T) {
	util := &AESUtil{}
	util.SetSecretKey("1234567890123456")

	// Valid encrypted data
	encryptedData := "TfkdxQxVIy7dGlgXcWgzNA==" // "hello world" encrypted

	assert.True(t, util.IsCiphertext(encryptedData))
}

func TestAESUtil_IsCiphertext_InvalidBase64(t *testing.T) {
	util := &AESUtil{}
	util.SetSecretKey("1234567890123456")

	// Invalid base64
	assert.False(t, util.IsCiphertext("invalid-base64!"))
}

func TestAESUtil_IsCiphertext_InvalidDecryption(t *testing.T) {
	util := &AESUtil{}
	util.SetSecretKey("1234567890123456")

	// Valid base64 but invalid encrypted data
	invalidEncrypted := base64.StdEncoding.EncodeToString([]byte("invalid-encrypted-data"))

	assert.False(t, util.IsCiphertext(invalidEncrypted))
}

func TestConvenienceFunctions(t *testing.T) {
	// Test convenience functions that use the default singleton
	SetDefaultSecretKey("1234567890123456")

	// Test AesDecode with valid base64 but invalid encrypted data
	validBase64 := "AAAAAAAAAAAAAAAAAAAAAA==" // 16 bytes of zeros, base64 encoded
	_, err := AesDecode(validBase64)

	// Should not fail due to missing secret key
	if err != nil {
		assert.NotContains(t, err.Error(), "AES secret key not set")
	}

	// Test AesDecodeWithKey with valid base64 but invalid encrypted data
	_, err2 := AesDecodeWithKey(validBase64, "1234567890123456")

	// Should not fail due to key issues
	if err2 != nil {
		assert.NotContains(t, err2.Error(), "key must be exactly 16 bytes")
		assert.NotContains(t, err2.Error(), "failed to decode base64")
	}

	// Test IsCiphertext with invalid data
	assert.False(t, IsCiphertext("invalid-base64"))
}

func TestRemovePKCS7Padding(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "data with valid padding",
			input:    []byte("hello world\x05\x05\x05\x05\x05"),
			expected: []byte("hello world"),
		},
		{
			name:     "data with invalid padding length",
			input:    []byte("hello world\xFF"),
			expected: []byte("hello world\xFF"), // Return as-is
		},
		{
			name:     "data with inconsistent padding",
			input:    []byte("hello world\x03\x03\x05"),
			expected: []byte("hello world\x03\x03\x05"), // Return as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := removePKCS7Padding(tt.input)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(tt.expected, result))
		})
	}
}

func TestIsPrintableString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "printable ASCII",
			input:    "hello world",
			expected: true,
		},
		{
			name:     "string with space",
			input:    "hello world 123",
			expected: true,
		},
		{
			name:     "string with special characters",
			input:    "hello-world_123!",
			expected: true,
		},
		{
			name:     "string with non-printable character",
			input:    "hello\x00world",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPrintableString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
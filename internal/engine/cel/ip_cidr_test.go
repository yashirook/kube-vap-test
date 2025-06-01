package cel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPLibraryFunctions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		expected   interface{}
		shouldErr  bool
	}{
		{
			name:       "isIP with valid IPv4",
			expression: "isIP('192.168.1.1')",
			expected:   true,
		},
		{
			name:       "isIP with invalid IP",
			expression: "isIP('192.168.1.256')",
			expected:   false,
		},
		{
			name:       "ip() function with valid IP",
			expression: "ip('10.0.0.1').family() == 4",
			expected:   true,
		},
		{
			name:       "ip() isLoopback check",
			expression: "ip('127.0.0.1').isLoopback()",
			expected:   true,
		},
		{
			name:       "ip() isGlobalUnicast check",
			expression: "ip('8.8.8.8').isGlobalUnicast()",
			expected:   true,
		},
		{
			name:       "isIP with variable",
			expression: "isIP(object.hostIP)",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"hostIP": "192.168.1.1",
				},
			},
			expected: true,
		},
	}

	evaluator, err := NewEvaluator()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, tt.variables)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCIDRLibraryFunctions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		expected   interface{}
		shouldErr  bool
	}{
		{
			name:       "isCIDR with valid CIDR",
			expression: "isCIDR('10.0.0.0/16')",
			expected:   true,
		},
		{
			name:       "isCIDR with invalid CIDR",
			expression: "isCIDR('10.0.0.0/33')",
			expected:   false,
		},
		{
			name:       "cidr containsIP",
			expression: "cidr('10.0.0.0/16').containsIP('10.0.1.1')",
			expected:   true,
		},
		{
			name:       "cidr containsIP outside range",
			expression: "cidr('10.0.0.0/16').containsIP('192.168.1.1')",
			expected:   false,
		},
		{
			name:       "cidr containsCIDR - same network",
			expression: "cidr('10.0.0.0/16').containsCIDR(cidr('10.0.0.0/24'))",
			expected:   true,
		},
		{
			name:       "cidr containsCIDR - different network",
			expression: "cidr('10.0.0.0/16').containsCIDR(cidr('10.1.0.0/24'))",
			expected:   false,
		},
		{
			name:       "cidr prefixLength",
			expression: "cidr('10.0.0.0/24').prefixLength() == 24",
			expected:   true,
		},
		{
			name:       "cidr masked",
			expression: "cidr('10.0.0.0/16') == cidr('10.0.0.0/16').masked()",
			expected:   true,
		},
		{
			name:       "cidr with variable",
			expression: "cidr('10.0.0.0/16').containsIP(object.podIP)",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"podIP": "10.0.1.5",
				},
			},
			expected: true,
		},
	}

	evaluator, err := NewEvaluator()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, tt.variables)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatLibraryFunctions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		variables  map[string]interface{}
		expected   interface{}
		shouldErr  bool
	}{
		{
			name:       "dns1123Label valid",
			expression: "format.dns1123Label().validate('my-valid-name').hasValue() == false",
			expected:   true,
		},
		{
			name:       "dns1123Label invalid",
			expression: "format.dns1123Label().validate('Invalid_Name').hasValue()",
			expected:   true,
		},
		{
			name:       "dns1123Subdomain valid",
			expression: "format.dns1123Subdomain().validate('api.example.com').hasValue() == false",
			expected:   true,
		},
		{
			name:       "uuid valid",
			expression: "format.uuid().validate('123e4567-e89b-12d3-a456-426614174000').hasValue() == false",
			expected:   true,
		},
		{
			name:       "uri valid",
			expression: "format.uri().validate('https://example.com').hasValue() == false",
			expected:   true,
		},
		{
			name:       "qualifiedName valid",
			expression: "format.qualifiedName().validate('apps/v1').hasValue() == false",
			expected:   true,
		},
		{
			name:       "format.named dns1123LabelPrefix",
			expression: `format.named("dns1123LabelPrefix").hasValue()`,
			expected:   true,
		},
		{
			name:       "format with variable",
			expression: "format.dns1123Label().validate(object.name).hasValue() == false",
			variables: map[string]interface{}{
				"object": map[string]interface{}{
					"name": "valid-name",
				},
			},
			expected: true,
		},
	}

	evaluator, err := NewEvaluator()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.expression, tt.variables)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
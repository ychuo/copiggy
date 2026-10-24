package clipmodel

import "testing"

func TestLabeledCodesRecognizeTypographyWithoutChangingOriginal(t *testing.T) {
	for _, tc := range []struct{ input, digits string }{
		{"OTP: １２３４５６", "123456"},
		{"Verification code: 123-456", "123456"},
		{"Authentication code: 1234 5678", "12345678"},
		{"2FA code = 12-34", "1234"},
		{"验证码：１２３－４５６", "123456"},
		{"校验码＝123\u202f456", "123456"},
		{"動態碼：１２３\u3000４５６", "123456"},
		{"確認コード: 123\u2011456", "123456"},
		{"ワンタイムパスワード：１２３４５６７８", "12345678"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			analysis := Analyze(tc.input)
			if analysis.Type != TypeOTP || analysis.Secret == nil || analysis.Secret.Value != tc.digits || analysis.Secret.Length != len(tc.digits) {
				t.Fatalf("code = %+v, want %s", analysis.Secret, tc.digits)
			}
			store := NewStore(1)
			item, ok := store.CaptureProtected(tc.input, "Fixture Editor", []string{"fixture.editor"})
			if !ok || item.Text != tc.input || item.Type != TypeOTP || !item.Allows("fixture.editor") || item.Allows("other.app") {
				t.Fatal("recognition changed the original or its protection")
			}
		})
	}
}

func TestCodesStillRequireWholeLabeledValues(t *testing.T) {
	for _, input := range []string{
		"１２３４５６", "123-456", "Your OTP is 123456", "OTP: 123456 expires soon",
		"OTP: 123", "OTP: 123456789", "OTP: 12345-6789", "OTP: 12--34", "OTP: 12 34 56",
		"OTP: 123\n456", "OTP: 123\u2028456", "OTP: 123\u2029456", "OTP: 123/456",
		"OTP: 123.456", "OTP: +123456", "OTP: -123456", "OTP: 12ab56",
	} {
		if got := Analyze(input); got.Type == TypeOTP || got.Secret != nil {
			t.Errorf("%q unexpectedly recognized: %+v", input, got)
		}
	}
}

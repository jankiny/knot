package policy

import "testing"

func TestEvaluateUsesMostRestrictiveAccessAtEveryLayer(t *testing.T) {
	none := AIAccessNone
	content := AIAccessContent
	read := LocalAccessRead
	readWrite := LocalAccessReadWrite

	tests := []struct {
		name      string
		input     Evaluation
		wantLocal LocalAccess
		wantAI    AIAccess
		reasons   []ReasonCode
	}{
		{
			name: "source restricts permissive defaults",
			input: Evaluation{
				Defaults: Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Source:   Access{LocalAccess: LocalAccessRead, AIAccess: AIAccessMetadata},
			},
			wantLocal: LocalAccessRead,
			wantAI:    AIAccessMetadata,
			reasons:   []ReasonCode{ReasonMetadataOnly},
		},
		{
			name: "project cannot widen source",
			input: Evaluation{
				Defaults: Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Source:   Access{LocalAccess: LocalAccessRead, AIAccess: AIAccessMetadata},
				Project:  &Override{LocalAccess: &readWrite, AIAccess: &content},
			},
			wantLocal: LocalAccessRead,
			wantAI:    AIAccessMetadata,
			reasons:   []ReasonCode{ReasonMetadataOnly},
		},
		{
			name: "document explicit deny wins",
			input: Evaluation{
				Defaults: Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Source:   Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Document: &Override{LocalAccess: &read, AIAccess: &none},
			},
			wantLocal: LocalAccessRead,
			wantAI:    AIAccessNone,
			reasons:   []ReasonCode{ReasonAIAccessDenied},
		},
		{
			name: "system restriction wins",
			input: Evaluation{
				Defaults: Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Source:   Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				System:   &Override{LocalAccess: &read, AIAccess: &none},
			},
			wantLocal: LocalAccessRead,
			wantAI:    AIAccessNone,
			reasons:   []ReasonCode{ReasonAIAccessDenied},
		},
		{
			name: "local deny also denies AI",
			input: Evaluation{
				Defaults: Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Source:   Access{LocalAccess: LocalAccessNone, AIAccess: AIAccessContent},
			},
			wantLocal: LocalAccessNone,
			wantAI:    AIAccessNone,
			reasons:   []ReasonCode{ReasonLocalAccessDenied, ReasonAIAccessDenied},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision, err := Evaluate(test.input)
			if err != nil {
				t.Fatalf("evaluate policy: %v", err)
			}
			if decision.LocalAccess != test.wantLocal || decision.AIAccess != test.wantAI {
				t.Fatalf("unexpected decision: %+v", decision)
			}
			for _, reason := range test.reasons {
				if !decision.HasReason(reason) {
					t.Fatalf("missing reason %q in %+v", reason, decision.Reasons)
				}
			}
		})
	}
}

func TestEvaluateSensitiveMarkersDenyAIWithoutDenyingLocalAccess(t *testing.T) {
	for _, path := range []string{
		`C:\Workspace\PhotoLibrary_NoAI\2026`,
		`c:/workspace/PRIVATE/contracts`,
		`资料\人员名单\2026`,
		`资料/个人信息表`,
	} {
		t.Run(path, func(t *testing.T) {
			decision, err := Evaluate(Evaluation{
				Defaults: Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Source:   Access{LocalAccess: LocalAccessReadWrite, AIAccess: AIAccessContent},
				Path:     path,
			})
			if err != nil {
				t.Fatalf("evaluate policy: %v", err)
			}
			if !decision.AllowsLocalWrite() {
				t.Fatalf("sensitive marker unexpectedly denied local management: %+v", decision)
			}
			if decision.AllowsAIMetadata() || !decision.HasReason(ReasonSensitivePathMarker) ||
				!decision.HasReason(ReasonAIAccessDenied) {
				t.Fatalf("sensitive marker did not deny AI: %+v", decision)
			}
		})
	}
}

func TestEvaluateRootStateAndMetadataReasons(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*Evaluation)
		reasons []ReasonCode
	}{
		{
			name: "disabled",
			change: func(input *Evaluation) {
				input.RootDisabled = true
			},
			reasons: []ReasonCode{
				ReasonRootDisabled,
				ReasonLocalAccessDenied,
				ReasonAIAccessDenied,
			},
		},
		{
			name: "offline",
			change: func(input *Evaluation) {
				input.RootOffline = true
			},
			reasons: []ReasonCode{
				ReasonRootOffline,
				ReasonLocalAccessDenied,
				ReasonAIAccessDenied,
			},
		},
		{
			name: "metadata",
			change: func(input *Evaluation) {
				input.Source.AIAccess = AIAccessMetadata
			},
			reasons: []ReasonCode{ReasonMetadataOnly},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := Evaluation{
				Defaults: Access{LocalAccess: LocalAccessRead, AIAccess: AIAccessContent},
				Source:   Access{LocalAccess: LocalAccessRead, AIAccess: AIAccessContent},
			}
			test.change(&input)
			decision, err := Evaluate(input)
			if err != nil {
				t.Fatalf("evaluate policy: %v", err)
			}
			for _, reason := range test.reasons {
				if !decision.HasReason(reason) {
					t.Fatalf("missing reason %q in %+v", reason, decision.Reasons)
				}
			}
		})
	}
}

func TestEvaluateRejectsInvalidEnums(t *testing.T) {
	_, err := Evaluate(Evaluation{
		Defaults: Access{LocalAccess: LocalAccessRead, AIAccess: AIAccessContent},
		Source:   Access{LocalAccess: LocalAccess("owner"), AIAccess: AIAccessContent},
	})
	if err == nil {
		t.Fatal("expected invalid local access to fail closed")
	}

	invalidAI := AIAccess("everything")
	_, err = Evaluate(Evaluation{
		Defaults: Access{LocalAccess: LocalAccessRead, AIAccess: AIAccessContent},
		Source:   Access{LocalAccess: LocalAccessRead, AIAccess: AIAccessContent},
		Document: &Override{AIAccess: &invalidAI},
	})
	if err == nil {
		t.Fatal("expected invalid AI access to fail closed")
	}
}

func TestParseLegacyAIAccess(t *testing.T) {
	tests := []struct {
		value   string
		want    AIAccess
		present bool
		wantErr bool
	}{
		{value: "", present: false},
		{value: "No_AI", want: AIAccessNone, present: true},
		{value: "private", want: AIAccessNone, present: true},
		{value: "metadata_only", want: AIAccessMetadata, present: true},
		{value: "content", want: AIAccessContent, present: true},
		{value: "surprise", present: true, wantErr: true},
	}
	for _, test := range tests {
		got, present, err := ParseLegacyAIAccess(test.value)
		if (err != nil) != test.wantErr || present != test.present || got != test.want {
			t.Fatalf(
				"ParseLegacyAIAccess(%q) = (%q, %t, %v)",
				test.value,
				got,
				present,
				err,
			)
		}
	}
}

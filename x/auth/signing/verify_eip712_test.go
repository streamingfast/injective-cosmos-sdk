package signing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

type eip712TestHandler struct {
	mode signingv1beta1.SignMode
}

func (h eip712TestHandler) Mode() signingv1beta1.SignMode {
	return h.mode
}

func (h eip712TestHandler) GetSignBytes(context.Context, txsigning.SignerData, txsigning.TxData) ([]byte, error) {
	return []byte(h.mode.String()), nil
}

func TestAPISignModesToInternalEIP712(t *testing.T) {
	modes, err := authsigning.APISignModesToInternal([]signingv1beta1.SignMode{
		signingv1beta1.SignMode_SIGN_MODE_EIP712_V2,
		signingv1beta1.SignMode_SIGN_MODE_EIP712_V3,
	})
	require.NoError(t, err)
	require.Equal(t, []signing.SignMode{
		signing.SignMode_SIGN_MODE_EIP712_V2,
		signing.SignMode_SIGN_MODE_EIP712_V3,
	}, modes)
}

func TestEIP712SignBytesAdapterAndVerification(t *testing.T) {
	for _, tc := range []struct {
		apiMode signingv1beta1.SignMode
		mode    signing.SignMode
	}{
		{signingv1beta1.SignMode_SIGN_MODE_EIP712_V2, signing.SignMode_SIGN_MODE_EIP712_V2},
		{signingv1beta1.SignMode_SIGN_MODE_EIP712_V3, signing.SignMode_SIGN_MODE_EIP712_V3},
	} {
		t.Run(tc.mode.String(), func(t *testing.T) {
			ctx := context.Background()
			handler := txsigning.NewHandlerMap(eip712TestHandler{mode: tc.apiMode})
			tx := moduletestutil.MakeTestEncodingConfig().TxConfig.NewTxBuilder().GetTx()
			txData := tx.(authsigning.V2AdaptableTx).GetSigningTxData()
			signBytes, err := authsigning.GetSignBytesAdapter(ctx, handler, tc.mode, authsigning.SignerData{}, tx)
			require.NoError(t, err)
			require.Equal(t, []byte(tc.apiMode.String()), signBytes)

			key := secp256k1.GenPrivKey()
			sig, err := key.Sign(signBytes)
			require.NoError(t, err)
			data := &signing.SingleSignatureData{SignMode: tc.mode, Signature: sig}
			require.NoError(t, authsigning.VerifySignature(ctx, key.PubKey(), txsigning.SignerData{}, data, handler, txData))

			data.Signature[0] ^= 1
			require.ErrorContains(t, authsigning.VerifySignature(ctx, key.PubKey(), txsigning.SignerData{}, data, handler, txData), "unable to verify single signer signature")
		})
	}
}

func TestVerifyEIP712Multisignature(t *testing.T) {
	for _, modes := range [][]signing.SignMode{
		{signing.SignMode_SIGN_MODE_EIP712_V3, signing.SignMode_SIGN_MODE_EIP712_V3},
		{signing.SignMode_SIGN_MODE_EIP712_V2, signing.SignMode_SIGN_MODE_EIP712_V3},
	} {
		t.Run(modes[0].String()+"/"+modes[1].String(), func(t *testing.T) {
			handler := txsigning.NewHandlerMap(
				eip712TestHandler{mode: signingv1beta1.SignMode_SIGN_MODE_EIP712_V2},
				eip712TestHandler{mode: signingv1beta1.SignMode_SIGN_MODE_EIP712_V3},
			)
			pubKeys := make([]cryptotypes.PubKey, len(modes))
			data := &signing.MultiSignatureData{BitArray: cryptotypes.NewCompactBitArray(len(modes))}
			for i, mode := range modes {
				key := secp256k1.GenPrivKey()
				pubKeys[i] = key.PubKey()
				sig, err := key.Sign([]byte(mode.String()))
				require.NoError(t, err)
				data.BitArray.SetIndex(i, true)
				data.Signatures = append(data.Signatures, &signing.SingleSignatureData{SignMode: mode, Signature: sig})
			}
			pubKey := multisig.NewLegacyAminoPubKey(len(modes), pubKeys)
			require.NoError(t, authsigning.VerifySignature(context.Background(), pubKey, txsigning.SignerData{}, data, handler, txsigning.TxData{}))

			data.Signatures[1].(*signing.SingleSignatureData).Signature[0] ^= 1
			require.Error(t, authsigning.VerifySignature(context.Background(), pubKey, txsigning.SignerData{}, data, handler, txsigning.TxData{}))
		})
	}
}

package state

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

	p_common "github.com/meta-node-blockchain/meta-node/pkg/common"
	"github.com/meta-node-blockchain/meta-node/pkg/logger"
)

func TestSmartContractState_Unmarshal(t *testing.T) {
	type fields struct {
		createPublicKey p_common.PublicKey
		storageAddress  common.Address
		codeHash        common.Hash
		storageRoot     common.Hash
	}
	type args struct {
		b []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:   "Test 1",
			fields: fields{},
			args: args{
				b: common.FromHex(
					"0a300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001a20ae779dfa8e07968602d8ee78cf4b925aec2f1431b327e4ee98e995f4f41e6e772220c8805329615a3ad861c7fb08ea437800eb61a588e64f3e4e4a62d7fba41bdcad3a14da7284fac5e804f8b9d71aa39310f0f86776b51d",
				),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := &SmartContractState{
				createPublicKey: tt.fields.createPublicKey,
				storageAddress:  tt.fields.storageAddress,
				codeHash:        tt.fields.codeHash,
				storageRoot:     tt.fields.storageRoot,
			}
			if err := ss.Unmarshal(tt.args.b); (err != nil) != tt.wantErr {
				t.Errorf("SmartContractState.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			logger.Info("ss: ", ss)
		})
	}
}

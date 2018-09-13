package pbft

import (
	"time"

	"gitlab.33.cn/chain33/chain33/common/merkle"
	"gitlab.33.cn/chain33/chain33/queue"
	drivers "gitlab.33.cn/chain33/chain33/system/consensus"
	pb "gitlab.33.cn/chain33/chain33/types"
)

func init() {
	drivers.Reg("pbft", NewPbft)
}

type PbftClient struct {
	*drivers.BaseClient
	replyChan   chan *pb.ClientReply
	requestChan chan *pb.Request
	isPrimary   bool
}

func NewBlockstore(cfg *pb.Consensus, replyChan chan *pb.ClientReply, requestChan chan *pb.Request, isPrimary bool) *PbftClient {
	c := drivers.NewBaseClient(cfg)
	client := &PbftClient{BaseClient: c, replyChan: replyChan, requestChan: requestChan, isPrimary: isPrimary}
	c.SetChild(client)
	return client
}
func (client *PbftClient) ProcEvent(msg queue.Message) bool {
	return false
}

func (client *PbftClient) Propose(block *pb.Block) {
	op := &pb.Operation{block}
	req := ToRequestClient(op, pb.Now().String(), client.BaseClient.Cfg.ClientAddr)
	client.requestChan <- req
}

func (client *PbftClient) CheckBlock(parent *pb.Block, current *pb.BlockDetail) error {
	return nil
}

func (client *PbftClient) SetQueueClient(c queue.Client) {
	plog.Info("Enter SetQueue method of pbft consensus")
	client.InitClient(c, func() {

		client.InitBlock()
	})
	go client.EventLoop()
	//go client.readReply()
	go client.CreateBlock()
}

func (client *PbftClient) CreateBlock() {
	issleep := true
	if !client.isPrimary {
		return
	}
	for {
		if issleep {
			time.Sleep(10 * time.Second)
		}
		plog.Info("=============start get tx===============")
		lastBlock := client.GetCurrentBlock()
		txs := client.RequestTx(int(pb.GetP(lastBlock.Height+1).MaxTxNumber), nil)
		if len(txs) == 0 {
			issleep = true
			continue
		}
		issleep = false
		plog.Info("==================start create new block!=====================")
		//check dup
		//txs = client.CheckTxDup(txs)
		//fmt.Println(len(txs))

		var newblock pb.Block
		newblock.ParentHash = lastBlock.Hash()
		newblock.Height = lastBlock.Height + 1
		newblock.Txs = txs
		newblock.TxHash = merkle.CalcMerkleRoot(newblock.Txs)
		newblock.BlockTime = pb.Now().Unix()
		if lastBlock.BlockTime >= newblock.BlockTime {
			newblock.BlockTime = lastBlock.BlockTime + 1
		}
		client.Propose(&newblock)
		//time.Sleep(time.Second)
		client.readReply()
		plog.Info("===============readreply and writeblock done===============")
	}
}

func (client *PbftClient) CreateGenesisTx() (ret []*pb.Transaction) {
	var tx pb.Transaction
	tx.Execer = []byte("coins")
	tx.To = client.Cfg.Genesis
	//gen payload
	g := &pb.CoinsAction_Genesis{}
	g.Genesis = &pb.CoinsGenesis{}
	g.Genesis.Amount = 1e8 * pb.Coin
	tx.Payload = pb.Encode(&pb.CoinsAction{Value: g, Ty: pb.CoinsActionGenesis})
	ret = append(ret, &tx)
	return
}

func (client *PbftClient) readReply() {

	data := <-client.replyChan
	if data == nil {
		plog.Error("block is nil")
		return
	}
	plog.Info("===============Get block from reply channel===========")
	//client.SetCurrentBlock(data.Result.Value)
	lastBlock := client.GetCurrentBlock()
	err := client.WriteBlock(lastBlock.StateHash, data.Result.Value)

	if err != nil {
		plog.Error("********************err:", err)
		return
	}
	client.SetCurrentBlock(data.Result.Value)

}
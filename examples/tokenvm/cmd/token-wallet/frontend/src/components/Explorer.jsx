import {useEffect, useState} from "react";
import {GetLatestBlocks,GetTransactionStats,GetAccountStats,GetUnitPrices,GetChainID} from "../../wailsjs/go/main/App";
import {Space, Typography, Divider, List, Card, Col, Row, Tooltip, message} from "antd";
import { Area, Line } from '@ant-design/plots';
const { Title, Text } = Typography;

const Explorer = () => {
    const [blocks, setBlocks] = useState([]);
    const [transactionStats, setTransactionStats] = useState([]);
    const [accountStats, setAccountStats] = useState([]);
    const [unitPrices, setUnitPrices] = useState([]);
    const [chainID, setChainID] = useState("");
    const [messageApi, contextHolder] = message.useMessage();

    useEffect(() => {
        const getLatestBlocks = async () => {
            GetLatestBlocks()
                .then((blocks) => {
                    setBlocks(blocks);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };

        const getChainID = async () => {
            GetChainID()
                .then((chainID) => {
                    setChainID(chainID);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };
        getChainID();

        const getTransactionStats = async () => {
            GetTransactionStats()
                .then((stats) => {
                    setTransactionStats(stats);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };

        const getAccountStats = async () => {
            GetAccountStats()
                .then((stats) => {
                    setAccountStats(stats);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };

        const getUnitPrices = async () => {
            GetUnitPrices()
                .then((prices) => {
                    setUnitPrices(prices);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };

        getLatestBlocks();
        getTransactionStats();
        getAccountStats();
        getUnitPrices();
        const interval = setInterval(() => {
          getLatestBlocks();
          getTransactionStats();
          getAccountStats();
          getUnitPrices();
        }, 500);

        return () => clearInterval(interval);
    }, []);

    return (<>
            {contextHolder}
            <Divider orientation="center">
              <Tooltip title="Last 2 Minutes">
                Metrics
              </Tooltip>
            </Divider>
  <Row gutter={16}>
    <Col span={8}>
      <Card title="Transactions Per Second" bordered={true}>
        <Area data={transactionStats} xField={"Timestamp"} yField={"Count"} autoFit={true} height={200} animation={false} xAxis={{tickCount: 0}} />
      </Card>
    </Col>
    <Col span={8}>
      <Card title="Active Accounts" bordered={true}>
        <Area data={accountStats} xField={"Timestamp"} yField={"Count"} autoFit={true} height={200} animation={false} xAxis={{tickCount: 0}} />
      </Card>
    </Col>
    <Col span={8}>
      <Card title="Unit Prices" bordered={true}>
        <Line data={unitPrices} xField={"Timestamp"} yField={"Count"} seriesField={"Category"} autoFit={true} height={200} animation={false} legend={false} xAxis={{tickCount: 0}} />
      </Card>
    </Col>
  </Row>
            <Divider orientation="center">
              <Tooltip title={`ChainID: ${chainID}`}>
                Recent Blocks
              </Tooltip>
            </Divider>
            <List
              bordered
              dataSource={blocks}
              renderItem={(item) => (
                <List.Item>
                  <div>
                    <Title level={3} style={{ display: "inline" }}>{item.Height}</Title> <Text type="secondary">{item.ID}</Text>
                  </div>
                  <Text strong>Timestamp:</Text> {item.Timestamp}
                  <br />
                  <Text strong>Transactions:</Text> {item.Txs}
                  {item.Txs > 0 &&
                    <Text italic type="danger"> (failed: {item.FailTxs})</Text>
                  }
                  <br />
                  <Text strong>Units Consumed:</Text> {item.Consumed}
                  <br />
                  <Text strong>State Root:</Text> {item.StateRoot}
                  <br />
                  <Text strong>Block Size:</Text> {item.Size}
                  <br />
                  <Text strong>Accept Latency:</Text> {item.Latency}ms
                </List.Item>
              )}
            />
        </>);
};

export default Explorer;

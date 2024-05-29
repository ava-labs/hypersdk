import { useEffect, useState } from "react";
import {
  GetLatestBlocks,
  GetTransactionStats,
  GetAccountStats,
  GetUnitPrices,
  GetChainID,
} from "../../wailsjs/go/main/App";
import { App, Typography, Divider, List, Card, Col, Row, Popover } from "antd";
import { InfoCircleOutlined } from "@ant-design/icons";
import { Area, Line } from "@ant-design/plots";
const { Title, Text } = Typography;

const Explorer = () => {
  const { message } = App.useApp();
  const [blocks, setBlocks] = useState([]);
  const [transactionStats, setTransactionStats] = useState([]);
  const [accountStats, setAccountStats] = useState([]);
  const [unitPrices, setUnitPrices] = useState([]);
  const [chainID, setChainID] = useState("");

  useEffect(() => {
    const getLatestBlocks = async () => {
      GetLatestBlocks()
        .then((blocks) => {
          setBlocks(blocks);
        })
        .catch((error) => {
          message.open({
            type: "error",
            content: error,
          });
        });
    };

    const getChainID = async () => {
      GetChainID()
        .then((chainID) => {
          setChainID(chainID);
        })
        .catch((error) => {
          message.open({
            type: "error",
            content: error,
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
          message.open({
            type: "error",
            content: error,
          });
        });
    };

    const getAccountStats = async () => {
      GetAccountStats()
        .then((stats) => {
          setAccountStats(stats);
        })
        .catch((error) => {
          message.open({
            type: "error",
            content: error,
          });
        });
    };

    const getUnitPrices = async () => {
      GetUnitPrices()
        .then((prices) => {
          setUnitPrices(prices);
        })
        .catch((error) => {
          message.open({
            type: "error",
            content: error,
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

  return (
    <>
      <Divider orientation="center">
        <Popover
          content={
            <div>
              <Text italic>
                Collection of TokenNet Telemetry Over the Last 2 Minutes
              </Text>
              <br />
              <br />
              <Text strong>Transactions Per Second:</Text> # of transactions
              accepted per second
              <br />
              <Text strong>Active Accounts:</Text> # of accounts issusing
              transactions
              <br />
              <Text strong>Unit Prices:</Text> Price of each HyperSDK fee
              dimension (Bandwidth, Compute, Storage[Read], Storage[Allocate],
              Storage[Write])
            </div>
          }>
          Metrics <InfoCircleOutlined />
        </Popover>
      </Divider>
      <Row gutter={16}>
        <Col span={8}>
          <Card title="Transactions Per Second" bordered={true}>
            <Area
              data={transactionStats}
              xField={"Timestamp"}
              yField={"Count"}
              autoFit={true}
              height={200}
              animation={false}
              xAxis={{ tickCount: 0 }}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card title="Active Accounts" bordered={true}>
            <Area
              data={accountStats}
              xField={"Timestamp"}
              yField={"Count"}
              autoFit={true}
              height={200}
              animation={false}
              xAxis={{ tickCount: 0 }}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card title="Unit Prices" bordered={true}>
            <Line
              data={unitPrices}
              xField={"Timestamp"}
              yField={"Count"}
              seriesField={"Category"}
              autoFit={true}
              height={200}
              animation={false}
              legend={false}
              xAxis={{ tickCount: 0 }}
            />
          </Card>
        </Col>
      </Row>
      <Divider orientation="center">
        <Popover
          content={
            <div>
              <Text italic>
                Recent activity for TokenNet (ChainID: {chainID})
              </Text>
              <br />
              <br />
              <Text strong>Timestamp:</Text> Time that block was created
              <br />
              <Text strong>Transactions:</Text> # of successful transactions in
              block
              <br />
              <Text strong>Units Consumed:</Text> # of HyperSDK fee units
              consumed
              <br />
              <Text strong>State Root:</Text> Merkle root of State at start of
              block execution
              <br />
              <Text strong>Block Size:</Text> Size of block in bytes
              <br />
              <Text strong>Accept Latency:</Text> Difference between block
              creation and block acceptance
            </div>
          }>
          Blocks <InfoCircleOutlined />
        </Popover>
      </Divider>
      <List
        bordered
        dataSource={blocks}
        renderItem={(item) => (
          <List.Item>
            <div>
              <Title level={3} style={{ display: "inline" }}>
                {item.Height}
              </Title>{" "}
              <Text type="secondary">{item.ID}</Text>
            </div>
            <Text strong>Timestamp:</Text> {item.Timestamp}
            <br />
            <Text strong>Transactions:</Text> {item.Txs}
            {item.Txs > 0 && (
              <Text italic type="danger">
                {" "}
                (failed: {item.FailTxs})
              </Text>
            )}
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
    </>
  );
};

export default Explorer;

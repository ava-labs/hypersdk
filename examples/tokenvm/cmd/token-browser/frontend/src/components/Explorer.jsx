import {useEffect, useState} from "react";
import {GetLatestBlocks,GetChainID} from "../../wailsjs/go/main/App";
import {Space, Typography, Divider, List, Card, Col, Row, message} from "antd";
import { Area } from '@ant-design/plots';
const { Title, Text } = Typography;

const Explorer = () => {
    const [blocks, setBlocks] = useState([]);
    const [tps, setTPS] = useState("0");
    const [chainID, setChainID] = useState("");
    const [prices, setPrices] = useState("");
    const [messageApi, contextHolder] = message.useMessage();

      const [data, setData] = useState([]);

    useEffect(() => {
        const getLatestBlocks = async () => {
            GetLatestBlocks()
                .then((blocks) => {
                    setBlocks(blocks);
                    if (blocks && blocks.length) {
                      setTPS(blocks[0].TPS);
                      setPrices(blocks[0].Prices);
                    }
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

        const asyncFetch = () => {
          fetch('https://gw.alipayobjects.com/os/bmw-prod/360c3eae-0c73-46f0-a982-4746a6095010.json')
            .then((response) => response.json())
            .then((json) => setData(json))
            .catch((error) => {
              console.log('fetch data failed', error);
            });
        };
        asyncFetch();

        const interval = setInterval(() => {
          getLatestBlocks();
        }, 500);

        return () => clearInterval(interval);
    }, []);

  const config = {
    data,
    xField: "timePeriod",
    yField: "value",
    autoFit: true,
    smooth: true,
    height: 200,
  };

    return (<>
            {contextHolder}
            <Divider orientation="center">Metrics</Divider>
            <br />
        <Row gutter={16}>
    <Col span={8}>
      <Card title="Transactions" bordered={true}>
        All-Time: [TODO] (TPS: {tps})
      </Card>
    </Col>
    <Col span={8}>
      <Card title="Accounts" bordered={true}>
        <Area {...config} />;
      </Card>
    </Col>
    <Col span={8}>
      <Card title="Unit Prices" bordered={true}>
        {prices}
      </Card>
    </Col>
  </Row>
            <Divider orientation="center">Blocks</Divider>
            <Row justify="center">
              <Text italic type="warning">ChainID: {chainID}</Text>
            </Row>
            <br />
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

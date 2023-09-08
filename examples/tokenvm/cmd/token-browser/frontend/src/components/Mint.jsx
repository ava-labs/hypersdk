import {useEffect, useState} from "react";
import {GetAssets, CreateAsset, GetBalance, GetAddress} from "../../wailsjs/go/main/App";
import { PlusOutlined } from "@ant-design/icons";
import { Input, Space, Typography, Divider, List, Card, Col, Row, Tooltip, Button, Modal, FloatButton, message } from "antd";
import { Area, Line } from '@ant-design/plots';
const { Title, Text } = Typography;

const Explorer = () => {
    const [assets, setAssets] = useState([]);
    const [tokenName, setTokenName] = useState("");
    const [address, setAddress] = useState("");
    const [balance, setBalance] = useState("");
    const [messageApi, contextHolder] = message.useMessage();
    const [isModalOpen, setIsModalOpen] = useState(false);

    const showModal = () => {
      setIsModalOpen(true);
    };

    const handleOk = () => {
      CreateAsset(tokenName)
          .then(() => {
              messageApi.open({
                  type: "success", content: "hello",
              });
          })
          .catch((error) => {
              messageApi.open({
                  type: "error", content: error,
              });
          });
      setTokenName("");
      setIsModalOpen(false);
    };

    const handleCancel = () => {
      setIsModalOpen(false);
    };

    useEffect(() => {
        const getAssets = async () => {
            GetAssets()
                .then((assets) => {
                    setAssets(assets);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };

        const getAddress = async () => {
            GetAddress()
                .then((address) => {
                    setAddress(address);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };
        getAddress();

        const getBalance = async () => {
            GetBalance("11111111111111111111111111111111LpoYY")
                .then((balance) => {
                    setBalance(balance);
                })
                .catch((error) => {
                    messageApi.open({
                        type: "error", content: error,
                    });
                });
        };

        const interval = setInterval(() => {
          getAssets();
          getBalance();
        }, 500);

        return () => clearInterval(interval);
    }, []);

    return (<>
            {contextHolder}
            <Text>{address}: {balance}</Text>
            <FloatButton icon={<PlusOutlined />} type="primary" onClick={showModal} />
            <Divider orientation="center">
              Tokens
            </Divider>
            <List
              bordered
              dataSource={assets}
              renderItem={(item) => (
                <List.Item>
                  <Title level={3}>{item.ID}</Title>
                </List.Item>
              )}
            />
            <Modal title="Create Your Own Token" open={isModalOpen} onOk={handleOk} onCancel={handleCancel} okText={"Create"}>
              <br />
              <Input placeholder={"Name"} allowClear={true} value={tokenName} onChange={(e) => setTokenName(e.target.value)} />
            </Modal>
        </>);
};

export default Explorer;

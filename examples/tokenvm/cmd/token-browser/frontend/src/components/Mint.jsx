import {useEffect, useState} from "react";
import {GetAssets, CreateAsset, GetBalance, GetAddress} from "../../wailsjs/go/main/App";
import { PlusOutlined } from "@ant-design/icons";
import { Input, Space, Typography, Divider, List, Card, Col, Row, Tooltip, Button, Drawer, FloatButton, Form, message } from "antd";
import { Area, Line } from '@ant-design/plots';
const { Title, Text } = Typography;

const Explorer = () => {
    const [assets, setAssets] = useState([]);
    const [address, setAddress] = useState("");
    const [balance, setBalance] = useState("");
    const [messageApi, contextHolder] = message.useMessage();
    const [open, setOpen] = useState(false);
    const [form] = Form.useForm();

    const showDrawer = () => {
      setOpen(true);
    };

    const onClose = () => {
      form.resetFields();
      setOpen(false);
    };

    const onFinish = (values) => {
      console.log('Success:', values);

      const start = (new Date()).getTime();
      CreateAsset(values.Symbol)
          .then(() => {
              const finish = (new Date()).getTime();
              messageApi.open({
                  type: "success", content: `Transaction Finalized (${finish-start} ms)`,
              });
          })
          .catch((error) => {
              messageApi.open({
                  type: "error", content: error,
              });
          });
      form.resetFields();
      setOpen(false);
    };
    
    const onFinishFailed = (errorInfo) => {
      console.log('Failed:', errorInfo);
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

        getAssets();
        getBalance();
        const interval = setInterval(() => {
          getAssets();
          getBalance();
        }, 500);

        return () => clearInterval(interval);
    }, []);

    return (<>
            {contextHolder}
            <FloatButton icon={<PlusOutlined />} type="primary" onClick={showDrawer} />
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
            <Drawer title={"Create a Token"} placement={"right"} onClose={onClose} open={open}>
              <Form
                name="basic"
                form={form}
                initialValues={{ remember: false }}
                onFinish={onFinish}
                onFinishFailed={onFinishFailed}
                autoComplete="off"
              >
                <Form.Item name="Symbol" rules={[{ required: true }]}>
                  <Input  placeholder="Symbol"/>
                </Form.Item>
                <Form.Item name="Metadata" rules={[{ required: true }]}>
                  <Input placeholder="Metadata"/>
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit">
                    Create
                  </Button>
                </Form.Item>
              </Form>
            </Drawer>
        </>);
};

export default Explorer;

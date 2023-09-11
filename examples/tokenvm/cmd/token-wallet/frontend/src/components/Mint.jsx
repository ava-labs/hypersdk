import {useEffect, useState} from "react";
import { GetMyAssets, CreateAsset, GetBalance, GetAddress } from "../../wailsjs/go/main/App";
import { PlusOutlined } from "@ant-design/icons";
import { Layout, Input, InputNumber, Space, Typography, Divider, List, Card, Col, Row, Tooltip, Button, Drawer, FloatButton, Form, message } from "antd";
import { Area, Line } from '@ant-design/plots';
const { Title, Text } = Typography;
const { Sider, Content } = Layout;

const Mint = () => {
    const [assets, setAssets] = useState([]);
    const [address, setAddress] = useState("");
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
      setOpen(false);

      (async () => {
        try {
          const start = (new Date()).getTime();
          await CreateAsset(values.Symbol, values.Decimals, values.Metadata);
          const finish = (new Date()).getTime();
          messageApi.open({
            type: "success", content: `Transaction Finalized (${finish-start} ms)`,
          });

          form.resetFields();
        } catch (e) {
          messageApi.open({
            type: "error", content: e.toString(),
          });
        }
      })();
    };
    
    const onFinishFailed = (errorInfo) => {
      console.log('Failed:', errorInfo);
    };

    useEffect(() => {
        const getMyAssets = async () => {
            const assets = await GetMyAssets();
            setAssets(assets);
        };

        getMyAssets();
        const interval = setInterval(() => {
          getMyAssets();
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
                  <Layout hasSider>
                  <Content style={{ backgroundColor: "white"}} >
                  <div>
                    <Title level={3} style={{ display: "inline" }}>{item.Symbol}</Title> <Text type="secondary">{item.ID}</Text>
                  </div>
                  <Text strong>Metadata:</Text> {item.Metadata}
                  <br />
                  <Text strong>Decimals:</Text> {item.Decimals} <Text strong>Supply:</Text> {item.Supply}
                  </Content>
                  <Sider style={{ backgroundColor: "white"}} >
                  <Button type="primary" style={{ width: "100%", height: "100%" }}>Mint</Button>
                  </Sider>
                  </Layout>
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
                  <Input placeholder="Symbol" maxLength="8"/>
                </Form.Item>
                <Form.Item name="Decimals" rules={[{ required: true }]}>
                  <InputNumber min={0} max={9} placeholder="Decimals" stringMode="true" style={{ width:"100%" }}/>
                </Form.Item>
                <Form.Item name="Metadata" rules={[{ required: true }]}>
                  <Input placeholder="Metadata" maxLength="256"/>
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

export default Mint;

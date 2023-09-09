import {useEffect, useState} from "react";
import {GetAssets, CreateAsset, GetBalance, GetAddress} from "../../wailsjs/go/main/App";
import { PlusOutlined } from "@ant-design/icons";
import { Input, Space, Typography, Divider, List, Card, Col, Row, Tooltip, Button, Drawer, FloatButton, Form, message } from "antd";
import { Area, Line } from '@ant-design/plots';
const { Title, Text } = Typography;

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
          const balance = await GetBalance("11111111111111111111111111111111LpoYY");
          if (balance != 0) {
            throw new Error("Insufficient Balance");
          }
          const start = (new Date()).getTime();
          await CreateAsset(values.Symbol);
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
        const getAssets = async () => {
            const assets = await GetAssets();
            setAssets(assets);
        };

        getAssets();
        const interval = setInterval(() => {
          getAssets();
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

export default Mint;

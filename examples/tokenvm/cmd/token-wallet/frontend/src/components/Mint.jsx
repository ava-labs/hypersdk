import {useEffect, useState} from "react";
import { GetMyAssets, CreateAsset, MintAsset, GetBalance, GetAddress } from "../../wailsjs/go/main/App";
import { PlusOutlined } from "@ant-design/icons";
import { App, Layout, Input, InputNumber, Space, Typography, Divider, List, Card, Col, Row, Tooltip, Button, Drawer, FloatButton, Form } from "antd";
import { Area, Line } from '@ant-design/plots';
const { Title, Text } = Typography;
const { Sider, Content } = Layout;

const Mint = () => {
    const { message } = App.useApp();
    const [assets, setAssets] = useState([]);
    const [address, setAddress] = useState("");
    const [openCreate, setOpenCreate] = useState(false);
    const [openMint, setOpenMint] = useState(false);
    const [mintFocus, setMintFocus] = useState({});
    const [createForm] = Form.useForm();
    const [mintForm] = Form.useForm();
    const key = "updatable";

    {/* Create Handlers */}
    const showCreateDrawer = () => {
      setOpenCreate(true);
    };

    const onCloseCreate = () => {
      createForm.resetFields();
      setOpenCreate(false);
    };

    const onFinishCreate = (values) => {
      console.log('Success:', values);
      createForm.resetFields();
      setOpenCreate(false);

      message.open({key, type: "loading", content: "Issuing Transaction...", duration:0});
      (async () => {
        try {
          const start = (new Date()).getTime();
          await CreateAsset(values.Symbol, values.Decimals, values.Metadata);
          const finish = (new Date()).getTime();
          message.open({
            key, type: "success", content: `Transaction Finalized (${finish-start} ms)`,
          });
        } catch (e) {
          message.open({
            key, type: "error", content: e.toString(),
          });
        }
      })();
    };
    
    const onFinishCreateFailed = (errorInfo) => {
      console.log('Failed:', errorInfo);
    };

    {/* Mint Handlers */}
    const showMintDrawer = (item) => {
      setMintFocus(item);
      setOpenMint(true);
    };

    const onCloseMint = () => {
      mintForm.resetFields();
      setOpenMint(false);
    };

    const onFillAddress = () => {
      mintForm.setFieldsValue({ Address: address });
    };

    const onFinishMint = (values) => {
      console.log('Success:', values);
      mintForm.resetFields();
      setOpenMint(false);

      message.open({key, type: "loading", content: "Issuing Transaction...", duration:0});
      (async () => {
        try {
          const start = (new Date()).getTime();
          await MintAsset(mintFocus.ID, values.Address, values.Amount);
          const finish = (new Date()).getTime();
          message.open({
            key, type: "success", content: `Transaction Finalized (${finish-start} ms)`,
          });
        } catch (e) {
          message.open({
            key, type: "error", content: e.toString(),
          });
        }
      })();
    };
    
    const onFinishMintFailed = (errorInfo) => {
      console.log('Failed:', errorInfo);
    };

    useEffect(() => {
        const getAddress = async () => {
          const address = await GetAddress();
          setAddress(address);
        };
        getAddress();

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
            <FloatButton icon={<PlusOutlined />} type="primary" onClick={showCreateDrawer} />
            <Divider orientation="center">
              Tokens
            </Divider>
            <List
              bordered
              dataSource={assets}
              renderItem={(item) => (
                <List.Item>
                  <div>
                    <Title level={3} style={{ display: "inline" }}>{item.Symbol}</Title> <Text type="secondary">{item.ID}</Text>
                  </div>
                  <Text strong>Decimals:</Text> {item.Decimals} <Text strong>Metadata:</Text> {item.Metadata}
                  <br />
                  <Text strong>Supply:</Text> {item.Supply}
                  <br />
                  <br />
                  <Button type="primary" style={{ width: "100%" }} onClick={() => showMintDrawer(item)}>Mint</Button>
                </List.Item>
              )}
            />
            <Drawer title={"Create a Token"} placement={"right"} onClose={onCloseCreate} open={openCreate}>
              <Form
                name="basic"
                form={createForm}
                initialValues={{ remember: false }}
                onFinish={onFinishCreate}
                onFinishFailed={onFinishCreateFailed}
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
            <Drawer title={`Mint ${mintFocus.Symbol}`} placement={"right"} onClose={onCloseMint} open={openMint}>
              <Form
                name="basic"
                form={mintForm}
                initialValues={{ remember: false }}
                onFinish={onFinishMint}
                onFinishFailed={onFinishMintFailed}
                autoComplete="off"
              >
                <Form.Item name="Address" rules={[{ required: true }]}>
                  <Input placeholder="Address" />
                </Form.Item>
                <Form.Item name="Amount" rules={[{ required: true }]}>
                  <InputNumber placeholder="Amount" min={0} stringMode="true" style={{ width:"100%" }}/>
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit">
                    Mint
                  </Button>
                  <Button type="link" htmlType="button" onClick={onFillAddress}>Populate Your Address</Button>
                </Form.Item>
              </Form>
            </Drawer>
        </>);
};

export default Mint;

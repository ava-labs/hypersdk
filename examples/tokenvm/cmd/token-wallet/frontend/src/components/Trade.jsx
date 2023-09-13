import {useEffect, useState} from "react";
import { FloatButton, App, Drawer, Divider, List, Card, Typography, Form, Input, InputNumber, Button, Select, Spin } from "antd";
import { CheckCircleTwoTone, CloseCircleTwoTone, LoadingOutlined, PlusOutlined, DoubleRightOutlined } from '@ant-design/icons';
import { StartFaucetSearch, GetFaucetSolutions } from "../../wailsjs/go/main/App";
const { Text, Title, Link } = Typography;

const Trade = () => {
    const { message } = App.useApp();
    const key = "updatable";

    {/* Create Handlers */}
    const [openCreate, setOpenCreate] = useState(false);
    const [createForm] = Form.useForm();
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

      message.open({key, type: "loading", content: "Processing Transaction...", duration:0});
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

    useEffect(() => {
      const interval = setInterval(() => {
      }, 500);

      return () => clearInterval(interval);
    }, []);

    return (<>
      <FloatButton icon={<PlusOutlined />} type="primary" onClick={showCreateDrawer} />
      <div style={{ width:"60%", margin: "auto" }}>
      <Divider orientation="center" >
        Open Orders
      </Divider>
      <List
        bordered
        dataSource={[]}
        renderItem={(item) => (
          <List.Item>
            <div>
              <Text strong>{item.Solution} </Text>
              <CloseCircleTwoTone twoToneColor="#eb2f96" />
            </div>
            <Text strong>Salt:</Text> {item.Salt}
            <br />
            <Text strong>Difficulty:</Text> {item.Difficulty}
            <br />
            <Text strong>Attempts:</Text> {item.Attempts}
            <br />
            <Text strong>Elapsed:</Text> {item.Elapsed}
            <br />
            <Text strong>Error:</Text> {item.Err}
          </List.Item>
        )}
      />
      <Divider orientation="center" >
        Order Book
      </Divider>
      <Select placeholder="In" style={{ width:"150px", margin: "0 8px 8px 0" }}/>
      <DoubleRightOutlined style={{ fontSize: "15px" }}/>
      <Select placeholder="Out" style={{ width:"150px", margin: "0 0 8px 8px" }}/>
      <List
        bordered
        dataSource={[]}
        renderItem={(item) => (
          <List.Item>
            <div>
              <Text strong>{item.Solution} </Text>
              <CloseCircleTwoTone twoToneColor="#eb2f96" />
            </div>
            <Text strong>Salt:</Text> {item.Salt}
            <br />
            <Text strong>Difficulty:</Text> {item.Difficulty}
            <br />
            <Text strong>Attempts:</Text> {item.Attempts}
            <br />
            <Text strong>Elapsed:</Text> {item.Elapsed}
            <br />
            <Text strong>Error:</Text> {item.Err}
          </List.Item>
        )}
      />
      </div>
      <Drawer title={"Create an Order"} placement={"right"} onClose={onCloseCreate} open={openCreate}>
        <Form
          name="basic"
          form={createForm}
          initialValues={{ remember: false }}
          onFinish={onFinishCreate}
          onFinishFailed={onFinishCreateFailed}
          autoComplete="off"
        >
          {/* inSymbol, inTick, outSymbol, outTick, supply (multiple of out tick) */}
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
}
export default Trade;

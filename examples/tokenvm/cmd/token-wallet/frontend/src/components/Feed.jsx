import { useEffect, useState } from "react";
import {
  GetFeedInfo,
  GetFeed,
  Message,
} from "../../wailsjs/go/main/App";
import FundsCheck from "./FundsCheck";
import { PlusOutlined } from "@ant-design/icons";
import {
  App,
  Select,
  Input,
  InputNumber,
  Space,
  Typography,
  Divider,
  List,
  Button,
  FloatButton,
  Drawer,
  Form,
} from "antd";
const { Title, Text } = Typography;

const Feed = () => {
  const { message } = App.useApp();
  const [feed, setFeed] = useState([]);
  const [feedInfo, setFeedInfo] = useState({});
  const [openCreate, setOpenCreate] = useState(false);
  const [createForm] = Form.useForm();
  const key = "updatable";

  {
    /* Create Handlers */
  }
  const showCreateDrawer = () => {
    setOpenCreate(true);
  };

  const onCloseCreate = () => {
    createForm.resetFields();
    setOpenCreate(false);
  };

  const onFinishCreate = (values) => {
    console.log("Success:", values);
    createForm.resetFields();
    setOpenCreate(false);

    message.open({
      key,
      type: "loading",
      content: "Processing Transaction...",
      duration: 0,
    });
    (async () => {
      try {
        const start = new Date().getTime();
        await Message(values.Message);
        const finish = new Date().getTime();
        message.open({
          key,
          type: "success",
          content: `Transaction Finalized (${finish - start} ms)`,
        });
      } catch (e) {
        message.open({
          key,
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  const onFinishCreateFailed = (errorInfo) => {
    console.log("Failed:", errorInfo);
  };

  useEffect(() => {
    const getFeed = async () => {
      const feed = await GetFeed();
      setFeed(feed);
    };
    const getFeedInfo = async () => {
      const feedInfo = await GetFeedInfo();
      setFeedInfo(feedInfo);
    };

    getFeed();
    getFeedInfo();
    const interval = setInterval(() => {
      getFeed();
      getFeedInfo();
    }, 500);

    return () => clearInterval(interval);
  }, []);

  return (
    <>
      <div style={{ width: "60%", margin: "auto" }}>
        <FundsCheck />
        <Divider orientation="center">Messages</Divider>
        <div style={{ display: "flex", width: "100%" }}>
          <Button
            type="primary"
            onClick={showCreateDrawer}
            placement={"right"}
            style={{ margin: "0 0 8px 0", "margin-left": "auto" }}
            disabled={!window.HasBalance}>
            Send Message
          </Button>
        </div>
        <List
          bordered
          dataSource={feed}
          renderItem={(item) => (
            <List.Item>
              <Title level={3} style={{ display: "inline" }}>{item.Memo}</Title>
              <br />
              <Text strong>TxID:</Text> {item.ID}
              <br />
              <Text strong>Timestamp:</Text> {item.Timestamp}
              <br />
              <Text strong>Fee:</Text> {item.Fee}
              <br />
              <Text strong>Actor:</Text> {item.Address}
            </List.Item>
          )}
        />
        <Divider orientation="center">Explanation</Divider>
      </div>
      <Drawer
        title={"Send Message"}
        placement={"right"}
        onClose={onCloseCreate}
        open={openCreate}>
        <Text strong>Feed Fee:</Text> ~{feedInfo.Fee}
        <Form
          name="basic"
          form={createForm}
          initialValues={{ remember: false }}
          onFinish={onFinishCreate}
          onFinishFailed={onFinishCreateFailed}
          style={{ margin: "8px 0 0 0" }}
          autoComplete="off">
          <Form.Item name="Message" style={{ margin: "0 0 8px 0" }} rules={[{ required: true }]}>
            <Input placeholder="Message" maxLength="256" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" style={{ margin: "0 0 8px 0" }}>
              Send
            </Button>
          </Form.Item>
        </Form>
        <Divider orientation="center">Explanation</Divider>
      </Drawer>
    </>
  );
};

export default Feed;

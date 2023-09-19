import { useEffect, useState } from "react";
import { GetFeedInfo, GetFeed, Message, OpenLink } from "../../wailsjs/go/main/App";
import FundsCheck from "./FundsCheck";
import { LinkOutlined, InfoCircleOutlined } from "@ant-design/icons";
import {
  App,
  Input,
  Typography,
  Divider,
  List,
  Button,
  Drawer,
  Form,
  Popover,
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
        await Message(values.Message, values.URL);
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

  const openLink = (url) => {
    OpenLink(url);
  }

  useEffect(() => {
    const getFeed = async () => {
      const feed = await GetFeed();
      console.log(feed);
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
        <Divider orientation="center">
          Messages
          <Popover content={"TODO: explanation"}>
            {" "}
            <InfoCircleOutlined />
          </Popover>
        </Divider>
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
              {item.URL.length == 0 &&
                <div>
                  <Title level={3} style={{ display: "inline" }}>
                    {item.Message}
                  </Title>
                  <br />
                </div>
              }
              {item.URL.length > 0 &&
                <div>
                {item.URLMeta != null &&
                  <div>
                    <img src={item.URLMeta.Image} style={{width:"100%", height: "200px", "object-fit":"cover"}}/>
                    <Title level={3} style={{ display: "inline" }}>
                      {item.URLMeta.Title}
                    </Title>
                    {" "}<Button onClick={() =>{openLink(item.URLMeta.URL)}} style={{margin: "0"}}><LinkOutlined /> {item.URLMeta.Host}</Button>
                    <br />
                    <Text italic>{item.URLMeta.Description}</Text>
                    <br />
                    <br />
                    <Text strong>Message:</Text> {item.Message}
                    <br />
                  </div>
                }
                {item.URLMeta == null &&
                  <div>
                    <Title level={3} style={{ display: "inline" }}>
                      {item.Message}
                    </Title>
                    <br />
                    <Text strong>URL:</Text> {item.URL} (<Text italic>not reachable</Text>)
                    <br />
                  </div>
                }
                </div>
              }
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
      </div>
      <Drawer
        title={
          <>
            Send Message
            <Popover content={"TODO: explanation"}>
              {" "}
              <InfoCircleOutlined />
            </Popover>
          </>
        }
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
          <Form.Item
            name="Message"
            style={{ margin: "0 0 8px 0" }}
            rules={[{ required: true }]}>
            <Input placeholder="Message" maxLength="256" />
          </Form.Item>
          <Form.Item
            name="URL"
            style={{ margin: "0 0 8px 0" }}>
            <Input placeholder="URL" maxLength="256" />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              style={{ margin: "0 0 8px 0" }}>
              Send
            </Button>
          </Form.Item>
        </Form>
      </Drawer>
    </>
  );
};

export default Feed;

import { useEffect, useState } from "react";
import {
  GetFeedInfo,
  GetFeed,
  Message,
  OpenLink,
  GetBalance,
  Transfer as Send,
} from "../../wailsjs/go/main/App";
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
  Select,
  InputNumber,
} from "antd";
const { Title, Text } = Typography;

const Feed = () => {
  const { message } = App.useApp();
  const [feed, setFeed] = useState([]);
  const [feedInfo, setFeedInfo] = useState({});
  const [openCreate, setOpenCreate] = useState(false);
  const [createForm] = Form.useForm();
  const [openTip, setOpenTip] = useState(false);
  const [tipFocus, setTipFocus] = useState({});
  const [tipForm] = Form.useForm();
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

  {
    /* Tip Handlers */
  }
  const [balance, setBalance] = useState([]);
  const getBalance = async () => {
    const bals = await GetBalance();
    const parsedBalances = [];
    for (let i = 0; i < bals.length; i++) {
      parsedBalances.push({ value: bals[i].ID, label: bals[i].Bal });
    }
    setBalance(parsedBalances);
  };
  const showTipDrawer = (item) => {
    setTipFocus(item);
    setOpenTip(true);
  };

  const onCloseTip = () => {
    tipForm.resetFields();
    setOpenTip(false);
  };

  const onFinishTip = (values) => {
    console.log("Success:", values);
    tipForm.resetFields();
    setOpenTip(false);

    message.open({
      key,
      type: "loading",
      content: "Processing Transaction...",
      duration: 0,
    });
    (async () => {
      try {
        const start = new Date().getTime();
        await Send(
          values.Asset,
          tipFocus.Address,
          values.Amount,
          `[${tipFocus.ID}]: ${values.Memo}`
        );
        const finish = new Date().getTime();
        message.open({
          key,
          type: "success",
          content: `Transaction Finalized (${finish - start} ms)`,
        });
        getBalance();
      } catch (e) {
        message.open({
          key,
          type: "error",
          content: e.toString(),
        });
      }
    })();
  };

  const onFinishTipFailed = (errorInfo) => {
    console.log("Failed:", errorInfo);
  };

  const openLink = (url) => {
    OpenLink(url);
  };

  useEffect(() => {
    getBalance();

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
          Posts
          <Popover
            content={
              <div>
                Because the fees are slow on TokenNet, it is great for
                micropayments.
                <br />
                <br />
                This example allows anyone to pay the feed operator to post
                content for everyone else to see.
                <br />
                <br />
                If the amount of posts goes above the target/5 minutes, the fee
                to post will increase.
                <br />
                <br />
                You can tip posters with any token you own!
              </div>
            }>
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
            Create Post
          </Button>
        </div>
        <List
          bordered
          dataSource={feed}
          renderItem={(item) => (
            <List.Item>
              {item.URL.length == 0 && (
                <div>
                  <Title level={3} style={{ display: "inline" }}>
                    {item.Message}
                  </Title>
                  <br />
                </div>
              )}
              {item.URL.length > 0 && (
                <div>
                  {item.URLMeta != null && (
                    <div>
                      {item.URLMeta.Image.length > 0 && (
                        <img
                          src={item.URLMeta.Image}
                          style={{
                            width: "100%",
                            height: "200px",
                            "object-fit": "cover",
                          }}
                        />
                      )}
                      <Title level={3} style={{ display: "inline" }}>
                        {item.URLMeta.Title}
                      </Title>{" "}
                      <Button
                        onClick={() => {
                          openLink(item.URLMeta.URL);
                        }}
                        style={{ margin: "0" }}>
                        <LinkOutlined /> {item.URLMeta.Host}
                      </Button>
                      <br />
                      {item.URLMeta.Description.length > 0 && (
                        <div>
                          <Text italic>{item.URLMeta.Description}</Text>
                          <br />
                          <br />
                        </div>
                      )}
                      <Text strong>URL:</Text> {item.URL}
                      <br />
                      <Text strong>Message:</Text> {item.Message}
                      <br />
                    </div>
                  )}
                  {item.URLMeta == null && (
                    <div>
                      <Title level={3} style={{ display: "inline" }}>
                        {item.Message}
                      </Title>
                      <br />
                      <Text strong>URL:</Text> {item.URL} (
                      <Text italic>not reachable</Text>)
                      <br />
                    </div>
                  )}
                </div>
              )}
              <Text strong>TxID:</Text> {item.ID}
              <br />
              <Text strong>Timestamp:</Text> {item.Timestamp}
              <br />
              <Text strong>Fee:</Text> {item.Fee}
              <br />
              <Text strong>Actor:</Text> {item.Address}
              <br />
              <Button
                type="primary"
                style={{ margin: "8px 0 0 0" }}
                disabled={!window.HasBalance}
                onClick={() => showTipDrawer(item)}>
                Tip
              </Button>
            </List.Item>
          )}
        />
      </div>
      <Drawer
        title={
          <>
            Create Post
            <Popover
              content={
                <div>
                  Posts can include both a "message" and a "URL".
                  <br />
                  <br />
                  When posting content, you must pay the "feed fee" to the
                  operator that hosts the feed (will be a HyperSDK Program in
                  the future).
                  <br />
                  <br />
                  The Token Wallet will attempt to preview posted URLs on the
                  feed.
                  <br />
                  <br />
                  The Token Wallet is not able to parse X or Reddit links yet.
                  If you'd like to implement that, please reach out!
                </div>
              }>
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
          <Form.Item name="URL" style={{ margin: "0 0 8px 0" }}>
            <Input placeholder="URL" maxLength="256" />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              style={{ margin: "0 0 8px 0" }}>
              Create
            </Button>
          </Form.Item>
        </Form>
      </Drawer>
      <Drawer
        title={
          <>
            Send Tip
            <Popover content={"TODO: explanation"}>
              {" "}
              <InfoCircleOutlined />
            </Popover>
          </>
        }
        placement={"right"}
        onClose={onCloseTip}
        open={openTip}>
        <Form
          name="basic"
          form={tipForm}
          initialValues={{ remember: false }}
          onFinish={onFinishTip}
          onFinishFailed={onFinishTipFailed}
          style={{ margin: "8px 0 0 0" }}
          autoComplete="off">
          <Form.Item
            name="Asset"
            rules={[{ required: true }]}
            style={{ margin: "0 0 8px 0" }}>
            <Select placeholder="Token" options={balance} />
          </Form.Item>
          <Form.Item
            name="Amount"
            rules={[{ required: true }]}
            style={{ margin: "0 0 8px 0" }}>
            <InputNumber
              placeholder="Amount"
              min={0}
              stringMode="true"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item
            name="Memo"
            rules={[{ required: false }]}
            style={{ margin: "0 0 8px 0" }}>
            <Input placeholder="Memo" maxLength="256" />
          </Form.Item>
          <Form.Item>
            <Button
              type="primary"
              htmlType="submit"
              style={{ margin: "0 0 8px 0" }}>
              Tip
            </Button>
          </Form.Item>
        </Form>
      </Drawer>
    </>
  );
};

export default Feed;

import NavBar from "./components/NavBar";
import { App as AApp, FloatButton, Layout, Row } from "antd";
import { Outlet } from "react-router-dom";
import logo from "./assets/images/hypersdk.png";

const { Content } = Layout;

const App = () => {
  return (
    <AApp>
      <Layout
        style={{
          minHeight: "95vh",
        }}
      >
        <NavBar />
        <Layout className="site-layout">
          <Content
            style={{
              background: "white",
              padding: "0 50px",
            }}
          >
            <div
              style={{
                padding: 24,
              }}
            >
              <Outlet />
              <FloatButton.BackTop />
            </div>
          </Content>
        </Layout>
        <Row justify="center" style= {{ background: "white" }}>
          <img src={logo} style={{ width: "300px" }} />
        </Row>
      </Layout>
    </AApp>
  );
};

export default App;

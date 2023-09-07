import App from "./App";

import Explorer from "./components/Explorer";

const routes = [
  {
    path: "/",
    element: <App />,
    children: [
      { index: true, element: <Explorer /> },
      {
        path: "explorer",
        element: <Explorer />,
      },
    ],
  },
];

export default routes;
